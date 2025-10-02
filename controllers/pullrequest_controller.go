package controllers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/google/go-github/v55/github"
	"golang.org/x/oauth2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"

	gitv1 "github.com/mihaigalos/git-change-operator/api/v1"
	"github.com/mihaigalos/git-change-operator/pkg/encryption"
	"github.com/mihaigalos/git-change-operator/pkg/jsonpath"
)

type PullRequestReconciler struct {
	client.Client
	Scheme           *runtime.Scheme
	metricsCollector *MetricsCollector
}

//+kubebuilder:rbac:groups=gco.galos.one,resources=pullrequests,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=gco.galos.one,resources=pullrequests/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=gco.galos.one,resources=pullrequests/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
//+kubebuilder:rbac:groups="*",resources="*",verbs=get;list;watch

func (r *PullRequestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Initialize metrics collector if not already set
	if r.metricsCollector == nil {
		r.metricsCollector = NewMetricsCollector("pullrequest")
	}

	var pullRequest gitv1.PullRequest
	if err := r.Get(ctx, req.NamespacedName, &pullRequest); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "unable to fetch PullRequest")
		return ctrl.Result{}, err
	}

	if pullRequest.Status.Phase == gitv1.PullRequestPhaseCreated {
		return ctrl.Result{}, nil
	}

	// Check REST API condition before proceeding
	shouldProceed, err := r.checkRestAPICondition(ctx, &pullRequest)
	if err != nil {
		log.Error(err, "failed to check REST API condition")
		r.updateStatus(ctx, &pullRequest, gitv1.PullRequestPhaseFailed, fmt.Sprintf("REST API condition check failed: %v", err))
		return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
	}

	if !shouldProceed {
		log.Info("REST API condition not met, skipping pull request creation")
		r.updateStatus(ctx, &pullRequest, gitv1.PullRequestPhasePending, "REST API condition not met, will retry later")
		return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
	}

	if err := r.updateStatus(ctx, &pullRequest, gitv1.PullRequestPhaseRunning, "Processing pull request"); err != nil {
		return ctrl.Result{}, err
	}

	auth, token, err := r.getAuthFromSecret(ctx, pullRequest.Namespace, pullRequest.Spec.AuthSecretRef, pullRequest.Spec.AuthSecretKey)
	if err != nil {
		log.Error(err, "failed to get authentication")
		r.updateStatus(ctx, &pullRequest, gitv1.PullRequestPhaseFailed, fmt.Sprintf("Authentication failed: %v", err))
		return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
	}

	prNumber, prURL, err := r.createPullRequest(ctx, &pullRequest, auth, token)
	if err != nil {
		log.Error(err, "failed to create pull request")
		r.updateStatus(ctx, &pullRequest, gitv1.PullRequestPhaseFailed, fmt.Sprintf("Pull request creation failed: %v", err))
		return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
	}

	pullRequest.Status.PullRequestNumber = prNumber
	pullRequest.Status.PullRequestURL = prURL
	if err := r.updateStatus(ctx, &pullRequest, gitv1.PullRequestPhaseCreated, "Pull request created successfully"); err != nil {
		return ctrl.Result{}, err
	}

	log.Info("Pull request created successfully", "number", prNumber, "url", prURL)
	return ctrl.Result{}, nil
}

func (r *PullRequestReconciler) fetchResource(ctx context.Context, resourceRef gitv1.ResourceRef, defaultNamespace string) (*unstructured.Unstructured, error) {
	gv, err := schema.ParseGroupVersion(resourceRef.ApiVersion)
	if err != nil {
		return nil, err
	}

	gvk := schema.GroupVersionKind{
		Group:   gv.Group,
		Version: gv.Version,
		Kind:    resourceRef.Kind,
	}

	resource := &unstructured.Unstructured{}
	resource.SetGroupVersionKind(gvk)

	namespace := resourceRef.Namespace
	if namespace == "" {
		namespace = defaultNamespace
	}

	namespacedName := types.NamespacedName{
		Name:      resourceRef.Name,
		Namespace: namespace,
	}

	if err := r.Get(ctx, namespacedName, resource); err != nil {
		return nil, err
	}

	return resource, nil
}

func (r *PullRequestReconciler) processResourceRef(ctx context.Context, resourceRef gitv1.ResourceRef, strategy gitv1.OutputStrategy, defaultNamespace string) (map[string][]byte, error) {
	resource, err := r.fetchResource(ctx, resourceRef, defaultNamespace)
	if err != nil {
		return nil, err
	}

	files := make(map[string][]byte)
	basePath := strategy.Path
	if basePath == "" {
		basePath = fmt.Sprintf("%s-%s", strings.ToLower(resourceRef.Kind), resourceRef.Name)
	}

	switch strategy.Type {
	case gitv1.OutputTypeDump:
		yamlData, err := yaml.Marshal(resource.Object)
		if err != nil {
			return nil, err
		}
		fileName := fmt.Sprintf("%s.yaml", basePath)
		files[fileName] = yamlData

	case gitv1.OutputTypeFields:
		if data, ok := resource.Object["data"].(map[string]interface{}); ok {
			for key, value := range data {
				fileName := filepath.Join(basePath, key)
				var content []byte
				if strValue, ok := value.(string); ok {
					content = []byte(strValue)
				} else {
					yamlValue, err := yaml.Marshal(value)
					if err != nil {
						return nil, err
					}
					content = yamlValue
				}
				files[fileName] = content
			}
		} else {
			return nil, fmt.Errorf("resource does not have a 'data' field suitable for fields extraction")
		}

	case gitv1.OutputTypeSingleField:
		if strategy.FieldRef == nil {
			return nil, fmt.Errorf("fieldRef is required for single-field output type")
		}

		var value interface{}
		var exists bool

		if data, ok := resource.Object["data"].(map[string]interface{}); ok {
			value, exists = data[strategy.FieldRef.Key]
		} else {
			value, exists = resource.Object[strategy.FieldRef.Key]
		}

		if !exists {
			return nil, fmt.Errorf("field %s not found in resource", strategy.FieldRef.Key)
		}

		fileName := basePath
		if strategy.FieldRef.FileName != "" {
			fileName = filepath.Join(filepath.Dir(basePath), strategy.FieldRef.FileName)
		}

		var content []byte
		if strValue, ok := value.(string); ok {
			content = []byte(strValue)
		} else {
			yamlValue, err := yaml.Marshal(value)
			if err != nil {
				return nil, err
			}
			content = yamlValue
		}
		files[fileName] = content

	default:
		return nil, fmt.Errorf("unsupported output type: %s", strategy.Type)
	}

	return files, nil
}

func (r *PullRequestReconciler) getAuthFromSecret(ctx context.Context, namespace, secretName, secretKey string) (*githttp.BasicAuth, string, error) {
	var secret corev1.Secret
	if err := r.Get(ctx, types.NamespacedName{Name: secretName, Namespace: namespace}, &secret); err != nil {
		return nil, "", err
	}

	key := secretKey
	if key == "" {
		key = "token"
	}

	token, exists := secret.Data[key]
	if !exists {
		return nil, "", fmt.Errorf("key %s not found in secret %s", key, secretName)
	}

	username := "oauth2"
	if usernameData, exists := secret.Data["username"]; exists {
		username = string(usernameData)
	}

	auth := &githttp.BasicAuth{
		Username: username,
		Password: string(token),
	}

	return auth, string(token), nil
}

func (r *PullRequestReconciler) createPullRequest(ctx context.Context, pr *gitv1.PullRequest, auth *githttp.BasicAuth, token string) (int, string, error) {
	tempDir, err := os.MkdirTemp("", "pull-request-")
	if err != nil {
		return 0, "", err
	}
	defer os.RemoveAll(tempDir)

	repo, err := git.PlainClone(tempDir, false, &git.CloneOptions{
		URL:  pr.Spec.Repository,
		Auth: auth,
	})
	if err != nil {
		return 0, "", err
	}

	w, err := repo.Worktree()
	if err != nil {
		return 0, "", err
	}

	branchRefName := plumbing.NewBranchReferenceName(pr.Spec.HeadBranch)
	b := plumbing.NewHashReference(branchRefName, plumbing.ZeroHash)

	err = w.Checkout(&git.CheckoutOptions{
		Branch: b.Name(),
		Create: true,
	})
	if err != nil {
		headRef, err := repo.Head()
		if err != nil {
			return 0, "", err
		}
		b = plumbing.NewHashReference(branchRefName, headRef.Hash())
		err = repo.Storer.SetReference(b)
		if err != nil {
			return 0, "", err
		}
		err = w.Checkout(&git.CheckoutOptions{Branch: b.Name()})
		if err != nil {
			return 0, "", err
		}
	}

	// Process regular files
	for _, file := range pr.Spec.Files {
		var content []byte

		// Determine content source
		if file.UseRestAPIData {
			// Use REST API response data
			if pr.Status.RestAPIStatus != nil && pr.Status.RestAPIStatus.FormattedOutput != "" {
				content = []byte(pr.Status.RestAPIStatus.FormattedOutput)
			} else {
				return 0, "", fmt.Errorf("file %s requested REST API data but no formatted output available", file.Path)
			}
		} else {
			// Use provided content
			content = []byte(file.Content)
		}

		filePath := filepath.Join(tempDir, file.Path)
		dir := filepath.Dir(filePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return 0, "", err
		}

		// Encrypt file content if encryption is enabled
		if encryption.ShouldEncryptFile(file.Path, pr.Spec.Encryption) {
			encryptedContent, err := r.encryptFileContent(ctx, content, pr.Spec.Encryption, pr.Namespace)
			if err != nil {
				return 0, "", fmt.Errorf("failed to encrypt file %s: %w", file.Path, err)
			}
			content = encryptedContent
			filePath = encryption.GetEncryptedFilePath(filePath, pr.Spec.Encryption)
		}

		if err := os.WriteFile(filePath, content, 0644); err != nil {
			return 0, "", err
		}

		// Add the correct file path to git (encrypted if applicable)
		gitPath := file.Path
		if encryption.ShouldEncryptFile(file.Path, pr.Spec.Encryption) {
			gitPath = encryption.GetEncryptedFilePath(file.Path, pr.Spec.Encryption)
		}
		if _, err := w.Add(gitPath); err != nil {
			return 0, "", err
		}
	}

	// Process resource references
	for _, resourceRef := range pr.Spec.ResourceRefs {
		files, err := r.processResourceRef(ctx, resourceRef, resourceRef.Strategy, pr.Namespace)
		if err != nil {
			return 0, "", fmt.Errorf("failed to process resource reference %s: %w", resourceRef.Name, err)
		}

		for relativePath, content := range files {
			filePath := filepath.Join(tempDir, relativePath)
			dir := filepath.Dir(filePath)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return 0, "", err
			}

			var finalContent []byte
			if resourceRef.Strategy.WriteMode == gitv1.WriteModeAppend {
				existingContent, _ := os.ReadFile(filePath)
				finalContent = append(existingContent, content...)
			} else {
				finalContent = content
			}

			// Encrypt content if encryption is enabled for this file
			if encryption.ShouldEncryptFile(relativePath, pr.Spec.Encryption) {
				encryptedContent, err := r.encryptFileContent(ctx, finalContent, pr.Spec.Encryption, pr.Namespace)
				if err != nil {
					return 0, "", fmt.Errorf("failed to encrypt file %s: %w", relativePath, err)
				}
				finalContent = encryptedContent
				filePath = encryption.GetEncryptedFilePath(filePath, pr.Spec.Encryption)
			}

			if err := os.WriteFile(filePath, finalContent, 0644); err != nil {
				return 0, "", err
			}

			// Add the correct file path to git (encrypted if applicable)
			gitPath := relativePath
			if encryption.ShouldEncryptFile(relativePath, pr.Spec.Encryption) {
				gitPath = encryption.GetEncryptedFilePath(relativePath, pr.Spec.Encryption)
			}
			if _, err := w.Add(gitPath); err != nil {
				return 0, "", err
			}
		}
	}

	commitMessage := fmt.Sprintf("Changes for PR: %s", pr.Spec.Title)
	_, err = w.Commit(commitMessage, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Git Change Operator",
			Email: "git-change-operator@galos.one",
			When:  time.Now(),
		},
	})
	if err != nil {
		return 0, "", err
	}

	err = repo.Push(&git.PushOptions{
		Auth: auth,
	})
	if err != nil {
		// Check if this is a non-fast-forward error (branch already exists)
		if strings.Contains(err.Error(), "non-fast-forward update") {
			// Branch already exists - this is fine, we can still try to create the PR
			// Log the situation but don't fail
			fmt.Printf("Branch %s already exists, proceeding to PR creation\n", pr.Spec.HeadBranch)
		} else {
			return 0, "", err
		}
	}

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	owner, repoName, err := r.parseRepository(pr.Spec.Repository)
	if err != nil {
		return 0, "", err
	}

	newPR := &github.NewPullRequest{
		Title:               github.String(pr.Spec.Title),
		Head:                github.String(pr.Spec.HeadBranch),
		Base:                github.String(pr.Spec.BaseBranch),
		Body:                github.String(pr.Spec.Body),
		MaintainerCanModify: github.Bool(true),
	}

	pullRequest, _, err := client.PullRequests.Create(ctx, owner, repoName, newPR)
	if err != nil {
		// Check for common GitHub API permission issues
		if strings.Contains(err.Error(), "Resource not accessible by personal access token") {
			return 0, "", fmt.Errorf("insufficient GitHub token permissions - token needs 'repo' and 'pull_requests:write' scopes to create pull requests: %w", err)
		}
		if strings.Contains(err.Error(), "403") || strings.Contains(err.Error(), "Forbidden") {
			return 0, "", fmt.Errorf("GitHub API access denied - check token permissions and repository access: %w", err)
		}
		return 0, "", fmt.Errorf("failed to create GitHub pull request: %w", err)
	}

	return pullRequest.GetNumber(), pullRequest.GetHTMLURL(), nil
}

func (r *PullRequestReconciler) parseRepository(repoURL string) (string, string, error) {
	if len(repoURL) < 19 {
		return "", "", fmt.Errorf("invalid repository URL")
	}

	if repoURL[:19] == "https://github.com/" {
		parts := repoURL[19:]
		if parts[len(parts)-4:] == ".git" {
			parts = parts[:len(parts)-4]
		}

		repoParts := filepath.Base(filepath.Dir("/" + parts))
		repoName := filepath.Base(parts)

		return repoParts, repoName, nil
	}

	return "", "", fmt.Errorf("unsupported repository URL format")
}

func (r *PullRequestReconciler) updateStatus(ctx context.Context, pr *gitv1.PullRequest, phase gitv1.PullRequestPhase, message string) error {
	pr.Status.Phase = phase
	pr.Status.Message = message
	now := metav1.Now()
	pr.Status.LastSync = &now

	return r.Status().Update(ctx, pr)
}

func (r *PullRequestReconciler) encryptFileContent(ctx context.Context, content []byte, encryptionConfig *gitv1.Encryption, namespace string) ([]byte, error) {
	if encryptionConfig == nil || !encryptionConfig.Enabled {
		return content, nil
	}

	// Resolve recipients (including secret references)
	resolvedRecipients, err := r.resolveRecipients(ctx, encryptionConfig.Recipients, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve recipients: %w", err)
	}

	// Create encryptor
	encryptor, err := encryption.NewEncryptor(resolvedRecipients)
	if err != nil {
		return nil, fmt.Errorf("failed to create encryptor: %w", err)
	}

	// Encrypt the content
	return encryptor.Encrypt(content)
}

func (r *PullRequestReconciler) resolveRecipients(ctx context.Context, recipients []gitv1.Recipient, namespace string) ([]gitv1.Recipient, error) {
	var resolved []gitv1.Recipient

	for _, recipient := range recipients {
		if recipient.SecretRef != nil {
			// Resolve value from secret
			var secret corev1.Secret
			if err := r.Get(ctx, types.NamespacedName{Name: recipient.SecretRef.Name, Namespace: namespace}, &secret); err != nil {
				return nil, fmt.Errorf("failed to get secret %s: %w", recipient.SecretRef.Name, err)
			}

			key := recipient.SecretRef.Key
			if key == "" {
				key = "publicKey"
			}

			value, exists := secret.Data[key]
			if !exists {
				return nil, fmt.Errorf("key %s not found in secret %s", key, recipient.SecretRef.Name)
			}

			resolved = append(resolved, gitv1.Recipient{
				Type:  recipient.Type,
				Value: string(value),
			})
		} else {
			resolved = append(resolved, recipient)
		}
	}

	return resolved, nil
}

func (r *PullRequestReconciler) checkRestAPICondition(ctx context.Context, pr *gitv1.PullRequest) (bool, error) {
	log := log.FromContext(ctx)

	if pr.Spec.RestAPI == nil {
		// No REST API condition specified, proceed with git operation
		return true, nil
	}

	restAPI := pr.Spec.RestAPI

	// Set defaults
	method := "GET"
	if restAPI.Method != "" {
		method = restAPI.Method
	}

	timeoutSeconds := 30
	if restAPI.TimeoutSeconds > 0 {
		timeoutSeconds = restAPI.TimeoutSeconds
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: time.Duration(timeoutSeconds) * time.Second,
	}

	// Create request
	var body io.Reader
	if restAPI.Body != "" {
		body = strings.NewReader(restAPI.Body)
	}

	req, err := http.NewRequestWithContext(ctx, method, restAPI.URL, body)
	if err != nil {
		return false, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Add headers
	for key, value := range restAPI.Headers {
		req.Header.Set(key, value)
	}

	// Add authentication if configured
	if restAPI.AuthSecretRef != "" {
		token, err := r.getTokenFromSecret(ctx, pr.Namespace, restAPI.AuthSecretRef, restAPI.AuthSecretKey)
		if err != nil {
			return false, fmt.Errorf("failed to get auth token: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}

	// Make the request
	startTime := time.Now()
	resp, err := client.Do(req)
	duration := time.Since(startTime)

	// Initialize status
	if pr.Status.RestAPIStatus == nil {
		pr.Status.RestAPIStatus = &gitv1.RestAPIStatus{}
	}

	now := metav1.Now()
	pr.Status.RestAPIStatus.LastCallTime = &now
	pr.Status.RestAPIStatus.CallCount++

	if err != nil {
		// Record failed request metrics
		r.metricsCollector.RecordAPIRequest(restAPI.URL, method, "error", duration, 0)
		pr.Status.RestAPIStatus.LastError = err.Error()
		log.Error(err, "REST API call failed", "url", restAPI.URL, "duration", duration)
		return false, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	pr.Status.RestAPIStatus.LastStatusCode = resp.StatusCode

	// Read full response body for processing
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		// Record metrics for successful HTTP but failed body read
		r.metricsCollector.RecordAPIRequest(restAPI.URL, method, fmt.Sprintf("%d", resp.StatusCode), duration, 0)
		pr.Status.RestAPIStatus.LastError = fmt.Sprintf("failed to read response: %v", err)
		return false, fmt.Errorf("failed to read response body: %w", err)
	}

	// Record successful request metrics
	r.metricsCollector.RecordAPIRequest(restAPI.URL, method, fmt.Sprintf("%d", resp.StatusCode), duration, int64(len(respBody)))

	// Store truncated response for status (max 1024 chars)
	if len(respBody) > 1024 {
		pr.Status.RestAPIStatus.LastResponse = string(respBody[:1024]) + "... (truncated)"
	} else {
		pr.Status.RestAPIStatus.LastResponse = string(respBody)
	}

	// Check HTTP status code first
	httpConditionMet := false
	if len(restAPI.ExpectedStatusCodes) > 0 {
		for _, expected := range restAPI.ExpectedStatusCodes {
			if resp.StatusCode == expected {
				httpConditionMet = true
				break
			}
		}
	} else {
		maxStatusCode := 399
		if restAPI.MaxStatusCode > 0 {
			maxStatusCode = restAPI.MaxStatusCode
		}
		httpConditionMet = resp.StatusCode <= maxStatusCode
	}

	if !httpConditionMet {
		r.metricsCollector.RecordConditionCheck("http_status_failed")
		pr.Status.RestAPIStatus.ConditionMet = false
		pr.Status.RestAPIStatus.LastError = fmt.Sprintf("HTTP status condition not met: %d", resp.StatusCode)
		log.Info("REST API HTTP status condition not met", "statusCode", resp.StatusCode)
		return false, nil
	}

	// Process JSON response if parsing is configured
	conditionMet := true
	if restAPI.ResponseParsing != nil {
		var err error
		conditionMet, err = r.processJSONResponse(ctx, pr, respBody, restAPI.ResponseParsing)
		if err != nil {
			r.metricsCollector.RecordJSONParsingError("processing_failed")
			pr.Status.RestAPIStatus.LastError = fmt.Sprintf("JSON processing failed: %v", err)
			log.Error(err, "Failed to process JSON response")
			return false, fmt.Errorf("JSON processing failed: %w", err)
		}
	}

	pr.Status.RestAPIStatus.ConditionMet = conditionMet
	pr.Status.RestAPIStatus.LastError = ""

	if conditionMet {
		pr.Status.RestAPIStatus.SuccessCount++
		r.metricsCollector.RecordConditionCheck("success")
	} else {
		r.metricsCollector.RecordConditionCheck("json_condition_failed")
	}

	// Update the PullRequest status
	if err := r.Status().Update(ctx, pr); err != nil {
		log.Error(err, "failed to update PullRequest status")
	}

	log.Info("REST API call completed",
		"url", restAPI.URL,
		"method", method,
		"statusCode", resp.StatusCode,
		"conditionMet", conditionMet,
		"duration", duration)

	return conditionMet, nil
}

// processJSONResponse processes the JSON response and extracts data according to the parsing configuration
func (r *PullRequestReconciler) processJSONResponse(ctx context.Context, pr *gitv1.PullRequest, respBody []byte, parsing *gitv1.ResponseParsing) (bool, error) {
	log := log.FromContext(ctx)

	// Check condition field if specified
	if parsing.ConditionField != "" && parsing.ConditionValue != "" {
		conditionValue, err := jsonpath.ExtractValue(respBody, parsing.ConditionField)
		if err != nil {
			r.metricsCollector.RecordJSONParsingError("condition_field_extraction_failed")
			return false, fmt.Errorf("failed to extract condition field %s: %w", parsing.ConditionField, err)
		}

		if conditionValue != parsing.ConditionValue {
			log.Info("JSON condition not met",
				"field", parsing.ConditionField,
				"expected", parsing.ConditionValue,
				"actual", conditionValue)
			return false, nil
		}

		log.Info("JSON condition met",
			"field", parsing.ConditionField,
			"value", conditionValue)
	}

	// Extract data fields if specified
	if len(parsing.DataFields) > 0 {
		extractedData, err := jsonpath.ExtractMultipleValues(respBody, parsing.DataFields)
		if err != nil {
			r.metricsCollector.RecordJSONParsingError("data_field_extraction_failed")
			return false, fmt.Errorf("failed to extract data fields: %w", err)
		}

		pr.Status.RestAPIStatus.ExtractedData = extractedData

		// Create formatted output
		separator := parsing.Separator
		if separator == "" {
			separator = ", "
		}

		var outputParts []string

		// Add timestamp if requested
		if parsing.IncludeTimestamp {
			timestamp := time.Now().Format(time.RFC3339)
			outputParts = append(outputParts, timestamp)
		}

		// Add extracted data
		outputParts = append(outputParts, extractedData...)

		pr.Status.RestAPIStatus.FormattedOutput = strings.Join(outputParts, separator)

		log.Info("Data extracted from JSON response",
			"dataFields", parsing.DataFields,
			"extractedData", extractedData,
			"formattedOutput", pr.Status.RestAPIStatus.FormattedOutput)
	}

	return true, nil
}

func (r *PullRequestReconciler) getTokenFromSecret(ctx context.Context, namespace, secretName, secretKey string) (string, error) {
	var secret corev1.Secret
	if err := r.Get(ctx, types.NamespacedName{Name: secretName, Namespace: namespace}, &secret); err != nil {
		return "", err
	}

	key := secretKey
	if key == "" {
		key = "token"
	}

	token, exists := secret.Data[key]
	if !exists {
		return "", fmt.Errorf("key %s not found in secret %s", key, secretName)
	}

	return string(token), nil
}

func (r *PullRequestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gitv1.PullRequest{}).
		Complete(r)
}
