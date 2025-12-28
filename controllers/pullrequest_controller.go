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
	"github.com/robfig/cron/v3"
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
	"github.com/mihaigalos/git-change-operator/pkg/cel"
	"github.com/mihaigalos/git-change-operator/pkg/encryption"
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

	// Check if scheduling is configured
	if pullRequest.Spec.Schedule != "" {
		return r.handleScheduledPullRequest(ctx, &pullRequest)
	}

	// Check if the resource has expired due to TTL (only for non-scheduled resources)
	expired, err := r.checkTTLExpired(ctx, &pullRequest)
	if err != nil {
		log.Error(err, "failed to check TTL expiration")
		return ctrl.Result{}, err
	}
	if expired {
		log.Info("Deleting expired PullRequest resource")
		if err := r.Delete(ctx, &pullRequest); err != nil {
			log.Error(err, "failed to delete expired PullRequest")
			return ctrl.Result{RequeueAfter: time.Minute * 1}, err
		}
		return ctrl.Result{}, nil
	}

	// For created resources, still requeue periodically for TTL checking
	if pullRequest.Status.Phase == gitv1.PullRequestPhaseCreated {
		return ctrl.Result{RequeueAfter: time.Minute * 1}, nil
	}

	// For failed resources, only check TTL - don't retry the operation
	// But still requeue periodically for TTL checking
	if pullRequest.Status.Phase == gitv1.PullRequestPhaseFailed {
		return ctrl.Result{RequeueAfter: time.Minute * 1}, nil
	}

	// Check REST API conditions before proceeding
	if len(pullRequest.Spec.RestAPIs) > 0 {
		shouldProceed, err := r.checkRestAPIConditions(ctx, &pullRequest)
		if err != nil {
			log.Error(err, "failed to check REST API conditions")
			r.updateStatus(ctx, &pullRequest, gitv1.PullRequestPhaseFailed, fmt.Sprintf("REST API condition check failed: %v", err))
			return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
		}

		if !shouldProceed {
			log.Info("One or more REST API conditions not met, skipping pull request creation")
			r.updateStatus(ctx, &pullRequest, gitv1.PullRequestPhasePending, "REST API condition not met, will retry later")
			return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
		}
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
			// Use REST API response data from multiple APIs
			content = r.buildFileContent(&file, pr.Status.RestAPIStatuses)
			if len(content) == 0 {
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

		// Handle writeMode for file content
		var finalContent []byte
		if file.WriteMode == gitv1.WriteModeAppend {
			existingContent, _ := os.ReadFile(filePath)
			finalContent = append(existingContent, content...)
		} else {
			finalContent = content
		}

		// Encrypt file content if encryption is enabled
		if encryption.ShouldEncryptFile(file.Path, pr.Spec.Encryption) {
			encryptedContent, err := r.encryptFileContent(ctx, finalContent, pr.Spec.Encryption, pr.Namespace)
			if err != nil {
				return 0, "", fmt.Errorf("failed to encrypt file %s: %w", file.Path, err)
			}
			finalContent = encryptedContent
			filePath = encryption.GetEncryptedFilePath(filePath, pr.Spec.Encryption)
		}

		if err := os.WriteFile(filePath, finalContent, 0644); err != nil {
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

// checkRestAPIConditions checks if all REST API conditions are met and extracts data for use in commits
func (r *PullRequestReconciler) checkRestAPIConditions(ctx context.Context, pr *gitv1.PullRequest) (bool, error) {
	log := log.FromContext(ctx)

	// Initialize status slice if needed
	if pr.Status.RestAPIStatuses == nil {
		pr.Status.RestAPIStatuses = make([]gitv1.RestAPIStatus, len(pr.Spec.RestAPIs))
	}

	// Ensure we have the right number of status entries
	if len(pr.Status.RestAPIStatuses) != len(pr.Spec.RestAPIs) {
		pr.Status.RestAPIStatuses = make([]gitv1.RestAPIStatus, len(pr.Spec.RestAPIs))
	}

	allConditionsMet := true

	// Process each REST API
	for i, restAPI := range pr.Spec.RestAPIs {
		conditionMet, err := r.checkSingleRestAPICondition(ctx, pr, &restAPI, &pr.Status.RestAPIStatuses[i])
		if err != nil {
			log.Error(err, "failed to check REST API condition", "name", restAPI.Name, "url", restAPI.URL)
			return false, err
		}

		if !conditionMet {
			allConditionsMet = false
			log.Info("REST API condition not met", "name", restAPI.Name, "url", restAPI.URL)
		} else {
			log.Info("REST API condition met", "name", restAPI.Name, "url", restAPI.URL)
		}
	}

	return allConditionsMet, nil
}

// checkSingleRestAPICondition checks if a single REST API condition is met
func (r *PullRequestReconciler) checkSingleRestAPICondition(ctx context.Context, pr *gitv1.PullRequest, restAPI *gitv1.RestAPI, status *gitv1.RestAPIStatus) (bool, error) {
	log := log.FromContext(ctx)

	// Set name in status
	status.Name = restAPI.Name

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

	now := metav1.Now()
	status.LastCallTime = &now
	status.CallCount++

	if err != nil {
		// Record failed request metrics
		r.metricsCollector.RecordAPIRequest(restAPI.URL, method, "error", duration, 0)
		status.LastError = err.Error()
		log.Error(err, "REST API call failed", "name", restAPI.Name, "url", restAPI.URL, "duration", duration)
		return false, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	status.LastStatusCode = resp.StatusCode

	// Read full response body for processing
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		// Record metrics for successful HTTP but failed body read
		r.metricsCollector.RecordAPIRequest(restAPI.URL, method, fmt.Sprintf("%d", resp.StatusCode), duration, 0)
		status.LastError = fmt.Sprintf("failed to read response: %v", err)
		return false, fmt.Errorf("failed to read response body: %w", err)
	}

	// Record successful request metrics
	r.metricsCollector.RecordAPIRequest(restAPI.URL, method, fmt.Sprintf("%d", resp.StatusCode), duration, int64(len(respBody)))

	// Store truncated response for status (max 1024 chars)
	if len(respBody) > 1024 {
		status.LastResponse = string(respBody[:1024]) + "... (truncated)"
	} else {
		status.LastResponse = string(respBody)
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
		status.ConditionMet = false
		status.LastError = fmt.Sprintf("HTTP status condition not met: %d", resp.StatusCode)
		log.Info("REST API HTTP status condition not met", "name", restAPI.Name, "statusCode", resp.StatusCode)
		return false, nil
	}

	// Process JSON response if parsing is configured
	conditionMet := true
	if restAPI.ResponseParsing != nil {
		var err error
		conditionMet, err = r.processJSONResponse(ctx, status, respBody, restAPI.ResponseParsing)
		if err != nil {
			r.metricsCollector.RecordJSONParsingError("processing_failed")
			status.LastError = fmt.Sprintf("JSON processing failed: %v", err)
			log.Error(err, "Failed to process JSON response", "name", restAPI.Name)
			return false, fmt.Errorf("JSON processing failed: %w", err)
		}
	}

	status.ConditionMet = conditionMet
	status.LastError = ""

	if conditionMet {
		status.SuccessCount++
		r.metricsCollector.RecordConditionCheck("success")
	} else {
		r.metricsCollector.RecordConditionCheck("json_condition_failed")
	}

	log.Info("REST API call completed",
		"name", restAPI.Name,
		"url", restAPI.URL,
		"method", method,
		"statusCode", resp.StatusCode,
		"conditionMet", conditionMet,
		"duration", duration)

	return conditionMet, nil
}

// processJSONResponse processes the JSON response and extracts data according to the parsing configuration using CEL
func (r *PullRequestReconciler) processJSONResponse(ctx context.Context, status *gitv1.RestAPIStatus, respBody []byte, parsing *gitv1.ResponseParsing) (bool, error) {
	log := log.FromContext(ctx)

	// Create CEL evaluator
	evaluator, err := cel.NewEvaluator()
	if err != nil {
		r.metricsCollector.RecordJSONParsingError("cel_evaluator_creation_failed")
		return false, fmt.Errorf("failed to create CEL evaluator: %w", err)
	}

	// Process response using CEL
	req := cel.ProcessRequest{
		Condition:      parsing.Condition,
		DataExpression: parsing.DataExpression,
		OutputFormat:   parsing.OutputFormat,
		ResponseData:   respBody,
	}

	result, err := evaluator.ProcessResponse(req)
	if err != nil {
		r.metricsCollector.RecordJSONParsingError("cel_processing_failed")
		return false, fmt.Errorf("failed to process JSON response with CEL: %w", err)
	}

	// Check if condition was met
	if !result.ConditionMet {
		log.Info("CEL condition not met",
			"condition", parsing.Condition,
			"response", string(respBody))
		return false, nil
	}

	log.Info("CEL condition met",
		"condition", parsing.Condition)

	// Update status with extracted data
	status.ExtractedData = result.ExtractedData
	status.FormattedOutput = result.FormattedOutput

	log.Info("Data extracted from JSON response using CEL",
		"dataExpression", parsing.DataExpression,
		"outputFormat", parsing.OutputFormat,
		"extractedData", result.ExtractedData,
		"formattedOutput", result.FormattedOutput)

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

// checkTTLExpired checks if the resource has expired based on TTL configuration
func (r *PullRequestReconciler) checkTTLExpired(ctx context.Context, pullRequest *gitv1.PullRequest) (bool, error) {
	log := log.FromContext(ctx)

	// If TTLMinutes is not set, no TTL expiration
	if pullRequest.Spec.TTLMinutes == nil {
		return false, nil
	}

	// Calculate expiration time
	creationTime := pullRequest.CreationTimestamp.Time
	ttlDuration := time.Duration(*pullRequest.Spec.TTLMinutes) * time.Minute
	expirationTime := creationTime.Add(ttlDuration)

	// Check if expired
	now := time.Now()
	if now.After(expirationTime) {
		log.Info("PullRequest resource has expired due to TTL",
			"creationTime", creationTime,
			"ttlMinutes", *pullRequest.Spec.TTLMinutes,
			"expirationTime", expirationTime,
			"currentTime", now)
		return true, nil
	}

	log.V(1).Info("PullRequest resource TTL check",
		"creationTime", creationTime,
		"ttlMinutes", *pullRequest.Spec.TTLMinutes,
		"expirationTime", expirationTime,
		"timeToExpiration", expirationTime.Sub(now))
	return false, nil
}

// buildFileContent builds the content for a file based on REST API data and configuration
func (r *PullRequestReconciler) buildFileContent(file *gitv1.File, statuses []gitv1.RestAPIStatus) []byte {
	if len(statuses) == 0 {
		return nil
	}

	var results []string

	// If a specific REST API name is specified, only use that one
	if file.RestAPIName != "" {
		for _, status := range statuses {
			if status.Name == file.RestAPIName && status.FormattedOutput != "" {
				results = append(results, status.FormattedOutput)
				break
			}
		}
	} else {
		// Use all successful REST API results
		for _, status := range statuses {
			if status.FormattedOutput != "" && status.ConditionMet {
				results = append(results, status.FormattedOutput)
			}
		}
	}

	if len(results) == 0 {
		return nil
	}

	// Get delimiter, default to newline
	delimiter := file.RestAPIDelimiter
	if delimiter == "" {
		delimiter = "\n"
	}

	// Join results with delimiter
	combinedContent := strings.Join(results, delimiter)
	return []byte(combinedContent)
}

// handleScheduledPullRequest processes PullRequest resources with a schedule configured
func (r *PullRequestReconciler) handleScheduledPullRequest(ctx context.Context, pullRequest *gitv1.PullRequest) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Check if execution is suspended
	if pullRequest.Spec.Suspend {
		log.Info("PullRequest execution is suspended")
		if err := r.updateScheduleStatus(ctx, pullRequest, gitv1.PullRequestPhasePending, "Execution suspended"); err != nil {
			return ctrl.Result{}, err
		}
		// Still requeue to check if suspend flag changes
		return ctrl.Result{RequeueAfter: time.Minute * 1}, nil
	}

	// Parse cron schedule
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	schedule, err := parser.Parse(pullRequest.Spec.Schedule)
	if err != nil {
		log.Error(err, "failed to parse cron schedule", "schedule", pullRequest.Spec.Schedule)
		if err := r.updateScheduleStatus(ctx, pullRequest, gitv1.PullRequestPhaseFailed, fmt.Sprintf("Invalid cron schedule: %v", err)); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, err
	}

	now := time.Now()

	// Calculate next scheduled time
	nextTime := schedule.Next(now)
	nextTimeMeta := metav1.NewTime(nextTime)

	// Check if it's time to execute
	shouldExecute := false
	if pullRequest.Status.LastScheduledTime == nil {
		// First execution - execute immediately
		shouldExecute = true
		log.Info("First scheduled execution, running immediately")
	} else {
		// Check if we've passed the next scheduled time
		if pullRequest.Status.NextScheduledTime != nil {
			scheduledTime := pullRequest.Status.NextScheduledTime.Time
			if now.After(scheduledTime) || now.Equal(scheduledTime) {
				shouldExecute = true
				log.Info("Scheduled time reached, executing", "scheduledTime", scheduledTime)
			}
		}
	}

	if !shouldExecute {
		// Not time to execute yet, update next scheduled time and requeue
		if pullRequest.Status.NextScheduledTime == nil || !pullRequest.Status.NextScheduledTime.Equal(&nextTimeMeta) {
			if err := r.updateNextScheduledTime(ctx, pullRequest, &nextTimeMeta); err != nil {
				return ctrl.Result{}, err
			}
		}

		// Calculate how long to wait until next execution
		waitDuration := time.Until(nextTime)
		if waitDuration < time.Minute {
			waitDuration = time.Minute
		}

		log.Info("Waiting for next scheduled execution", "nextTime", nextTime, "waitDuration", waitDuration)
		return ctrl.Result{RequeueAfter: waitDuration}, nil
	}

	// Time to execute - update last scheduled time
	nowMeta := metav1.NewTime(now)
	pullRequest.Status.LastScheduledTime = &nowMeta

	// Execute the pull request creation
	log.Info("Executing scheduled PullRequest")

	// Update status to Running
	if err := r.updateScheduleStatus(ctx, pullRequest, gitv1.PullRequestPhaseRunning, "Processing scheduled pull request"); err != nil {
		return ctrl.Result{}, err
	}

	// Check REST API conditions if configured
	if len(pullRequest.Spec.RestAPIs) > 0 {
		shouldProceed, err := r.checkRestAPIConditions(ctx, pullRequest)
		if err != nil {
			log.Error(err, "failed to check REST API conditions")
			r.recordPRExecution(ctx, pullRequest, 0, "", gitv1.PullRequestPhaseFailed, fmt.Sprintf("REST API condition check failed: %v", err))
			// Calculate next execution time
			nextTime := schedule.Next(now)
			nextTimeMeta := metav1.NewTime(nextTime)
			r.updateNextScheduledTime(ctx, pullRequest, &nextTimeMeta)
			return ctrl.Result{RequeueAfter: time.Until(nextTime)}, nil
		}

		if !shouldProceed {
			log.Info("REST API conditions not met, skipping this scheduled execution")
			r.recordPRExecution(ctx, pullRequest, 0, "", gitv1.PullRequestPhasePending, "REST API conditions not met")
			// Calculate next execution time
			nextTime := schedule.Next(now)
			nextTimeMeta := metav1.NewTime(nextTime)
			r.updateNextScheduledTime(ctx, pullRequest, &nextTimeMeta)
			return ctrl.Result{RequeueAfter: time.Until(nextTime)}, nil
		}

		log.Info("All REST API conditions met, proceeding with scheduled pull request")
	}

	auth, token, err := r.getAuthFromSecret(ctx, pullRequest.Namespace, pullRequest.Spec.AuthSecretRef, pullRequest.Spec.AuthSecretKey)
	if err != nil {
		log.Error(err, "failed to get authentication")
		r.recordPRExecution(ctx, pullRequest, 0, "", gitv1.PullRequestPhaseFailed, fmt.Sprintf("Authentication failed: %v", err))
		// Calculate next execution time
		nextTime := schedule.Next(now)
		nextTimeMeta := metav1.NewTime(nextTime)
		r.updateNextScheduledTime(ctx, pullRequest, &nextTimeMeta)
		return ctrl.Result{RequeueAfter: time.Until(nextTime)}, nil
	}

	prNumber, prURL, err := r.createPullRequest(ctx, pullRequest, auth, token)
	if err != nil {
		log.Error(err, "failed to create scheduled pull request")
		r.recordPRExecution(ctx, pullRequest, 0, "", gitv1.PullRequestPhaseFailed, fmt.Sprintf("Pull request creation failed: %v", err))
		// Calculate next execution time
		nextTime := schedule.Next(now)
		nextTimeMeta := metav1.NewTime(nextTime)
		r.updateNextScheduledTime(ctx, pullRequest, &nextTimeMeta)
		return ctrl.Result{RequeueAfter: time.Until(nextTime)}, nil
	}

	// Record successful execution
	log.Info("Scheduled pull request created successfully", "prNumber", prNumber, "prURL", prURL)
	r.recordPRExecution(ctx, pullRequest, prNumber, prURL, gitv1.PullRequestPhaseCreated, "Pull request created successfully")

	// Calculate next execution time
	nextTime = schedule.Next(now)
	nextTimeMeta = metav1.NewTime(nextTime)
	r.updateNextScheduledTime(ctx, pullRequest, &nextTimeMeta)

	// Requeue for next execution
	waitDuration := time.Until(nextTime)
	if waitDuration < time.Minute {
		waitDuration = time.Minute
	}

	log.Info("Scheduled execution complete, waiting for next run", "nextTime", nextTime, "waitDuration", waitDuration)
	return ctrl.Result{RequeueAfter: waitDuration}, nil
}

// recordPRExecution adds an execution record to the history and maintains the max history limit
func (r *PullRequestReconciler) recordPRExecution(ctx context.Context, pullRequest *gitv1.PullRequest, prNumber int, prURL string, phase gitv1.PullRequestPhase, message string) error {
	log := log.FromContext(ctx)

	// Retry logic to handle optimistic concurrency conflicts
	const maxRetries = 3
	for i := 0; i < maxRetries; i++ {
		// Get fresh copy of the resource
		fresh := &gitv1.PullRequest{}
		if err := r.Get(ctx, client.ObjectKeyFromObject(pullRequest), fresh); err != nil {
			return err
		}

		// Create new execution record
		now := metav1.Now()
		record := gitv1.PRExecutionRecord{
			ExecutionTime:     now,
			PullRequestNumber: prNumber,
			PullRequestURL:    prURL,
			Phase:             phase,
			Message:           message,
		}

		// Add to execution history
		fresh.Status.ExecutionHistory = append([]gitv1.PRExecutionRecord{record}, fresh.Status.ExecutionHistory...)

		// Maintain max history limit
		maxHistory := 10 // default
		if pullRequest.Spec.MaxExecutionHistory != nil {
			maxHistory = *pullRequest.Spec.MaxExecutionHistory
		}
		if len(fresh.Status.ExecutionHistory) > maxHistory {
			fresh.Status.ExecutionHistory = fresh.Status.ExecutionHistory[:maxHistory]
		}

		// Update current status fields
		fresh.Status.Phase = phase
		fresh.Status.Message = message
		fresh.Status.PullRequestNumber = prNumber
		fresh.Status.PullRequestURL = prURL
		fresh.Status.LastSync = &now
		fresh.Status.LastScheduledTime = pullRequest.Status.LastScheduledTime

		// Attempt to update status
		if err := r.Status().Update(ctx, fresh); err != nil {
			if errors.IsConflict(err) && i < maxRetries-1 {
				log.V(1).Info("Status update conflict, retrying", "attempt", i+1)
				time.Sleep(time.Millisecond * 100)
				continue
			}
			return err
		}

		// Update successful, copy status back to original object
		pullRequest.Status = fresh.Status
		return nil
	}

	return fmt.Errorf("failed to update status after %d retries", maxRetries)
}

// updateScheduleStatus updates the status with schedule-aware information
func (r *PullRequestReconciler) updateScheduleStatus(ctx context.Context, pullRequest *gitv1.PullRequest, phase gitv1.PullRequestPhase, message string) error {
	log := log.FromContext(ctx)

	const maxRetries = 3
	for i := 0; i < maxRetries; i++ {
		fresh := &gitv1.PullRequest{}
		if err := r.Get(ctx, client.ObjectKeyFromObject(pullRequest), fresh); err != nil {
			return err
		}

		fresh.Status.Phase = phase
		fresh.Status.Message = message
		now := metav1.Now()
		fresh.Status.LastSync = &now

		// Preserve schedule-related fields
		if pullRequest.Status.LastScheduledTime != nil {
			fresh.Status.LastScheduledTime = pullRequest.Status.LastScheduledTime
		}
		if pullRequest.Status.NextScheduledTime != nil {
			fresh.Status.NextScheduledTime = pullRequest.Status.NextScheduledTime
		}

		// Copy over REST API statuses if they exist
		if len(pullRequest.Status.RestAPIStatuses) > 0 {
			fresh.Status.RestAPIStatuses = pullRequest.Status.RestAPIStatuses
		}

		// Copy over execution history
		if len(pullRequest.Status.ExecutionHistory) > 0 {
			fresh.Status.ExecutionHistory = pullRequest.Status.ExecutionHistory
		}

		if err := r.Status().Update(ctx, fresh); err != nil {
			if errors.IsConflict(err) && i < maxRetries-1 {
				log.V(1).Info("Status update conflict, retrying", "attempt", i+1)
				time.Sleep(time.Millisecond * 100)
				continue
			}
			return err
		}

		pullRequest.Status = fresh.Status
		return nil
	}

	return fmt.Errorf("failed to update status after %d retries", maxRetries)
}

// updateNextScheduledTime updates only the next scheduled time field
func (r *PullRequestReconciler) updateNextScheduledTime(ctx context.Context, pullRequest *gitv1.PullRequest, nextTime *metav1.Time) error {
	log := log.FromContext(ctx)

	const maxRetries = 3
	for i := 0; i < maxRetries; i++ {
		fresh := &gitv1.PullRequest{}
		if err := r.Get(ctx, client.ObjectKeyFromObject(pullRequest), fresh); err != nil {
			return err
		}

		fresh.Status.NextScheduledTime = nextTime

		if err := r.Status().Update(ctx, fresh); err != nil {
			if errors.IsConflict(err) && i < maxRetries-1 {
				log.V(1).Info("Status update conflict, retrying", "attempt", i+1)
				time.Sleep(time.Millisecond * 100)
				continue
			}
			return err
		}

		pullRequest.Status.NextScheduledTime = nextTime
		return nil
	}

	return fmt.Errorf("failed to update next scheduled time after %d retries", maxRetries)
}

func (r *PullRequestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gitv1.PullRequest{}).
		Complete(r)
}
