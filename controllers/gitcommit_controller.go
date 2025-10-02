package controllers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
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

type GitCommitReconciler struct {
	client.Client
	Scheme           *runtime.Scheme
	metricsCollector *MetricsCollector
}

//+kubebuilder:rbac:groups=gco.galos.one,resources=gitcommits,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=gco.galos.one,resources=gitcommits/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=gco.galos.one,resources=gitcommits/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

func (r *GitCommitReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Initialize metrics collector if not already set
	if r.metricsCollector == nil {
		r.metricsCollector = NewMetricsCollector("gitcommit")
	}

	var gitCommit gitv1.GitCommit
	if err := r.Get(ctx, req.NamespacedName, &gitCommit); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "unable to fetch GitCommit")
		return ctrl.Result{}, err
	}

	if gitCommit.Status.Phase == gitv1.GitCommitPhaseCommitted {
		return ctrl.Result{}, nil
	}

	if err := r.updateStatus(ctx, &gitCommit, gitv1.GitCommitPhaseRunning, "Processing git commit"); err != nil {
		return ctrl.Result{}, err
	}

	// Check REST API condition if configured
	if gitCommit.Spec.RestAPI != nil {
		conditionMet, err := r.checkRestAPICondition(ctx, &gitCommit)
		if err != nil {
			log.Error(err, "failed to check REST API condition")
			r.updateStatus(ctx, &gitCommit, gitv1.GitCommitPhaseFailed, fmt.Sprintf("REST API check failed: %v", err))
			return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
		}

		if !conditionMet {
			log.Info("REST API condition not met, skipping git commit")
			r.updateStatus(ctx, &gitCommit, gitv1.GitCommitPhasePending, "REST API condition not met, waiting...")
			return ctrl.Result{RequeueAfter: time.Minute * 2}, nil
		}

		log.Info("REST API condition met, proceeding with git commit")
	}

	auth, err := r.getAuthFromSecret(ctx, gitCommit.Namespace, gitCommit.Spec.AuthSecretRef, gitCommit.Spec.AuthSecretKey)
	if err != nil {
		log.Error(err, "failed to get authentication")
		r.updateStatus(ctx, &gitCommit, gitv1.GitCommitPhaseFailed, fmt.Sprintf("Authentication failed: %v", err))
		return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
	}

	commitSHA, err := r.performGitCommit(ctx, &gitCommit, auth)
	if err != nil {
		log.Error(err, "failed to perform git commit")
		r.updateStatus(ctx, &gitCommit, gitv1.GitCommitPhaseFailed, fmt.Sprintf("Git commit failed: %v", err))
		return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
	}

	gitCommit.Status.CommitSHA = commitSHA
	if err := r.updateStatus(ctx, &gitCommit, gitv1.GitCommitPhaseCommitted, "Git commit completed successfully"); err != nil {
		return ctrl.Result{}, err
	}

	log.Info("Git commit completed successfully", "commit", commitSHA)
	return ctrl.Result{}, nil
}

func (r *GitCommitReconciler) getAuthFromSecret(ctx context.Context, namespace, secretName, secretKey string) (*githttp.BasicAuth, error) {
	var secret corev1.Secret
	if err := r.Get(ctx, types.NamespacedName{Name: secretName, Namespace: namespace}, &secret); err != nil {
		return nil, err
	}

	key := secretKey
	if key == "" {
		key = "token"
	}

	token, exists := secret.Data[key]
	if !exists {
		return nil, fmt.Errorf("key %s not found in secret %s", key, secretName)
	}

	username := "oauth2"
	if usernameData, exists := secret.Data["username"]; exists {
		username = string(usernameData)
	}

	return &githttp.BasicAuth{
		Username: username,
		Password: string(token),
	}, nil
}

func (r *GitCommitReconciler) performGitCommit(ctx context.Context, gitCommit *gitv1.GitCommit, auth *githttp.BasicAuth) (string, error) {
	tempDir, err := ioutil.TempDir("", "git-commit-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tempDir)

	repo, err := git.PlainClone(tempDir, false, &git.CloneOptions{
		URL:  gitCommit.Spec.Repository,
		Auth: auth,
	})
	if err != nil {
		return "", err
	}

	w, err := repo.Worktree()
	if err != nil {
		return "", err
	}

	if gitCommit.Spec.Branch != "" && gitCommit.Spec.Branch != "main" && gitCommit.Spec.Branch != "master" {
		branchRefName := plumbing.NewBranchReferenceName(gitCommit.Spec.Branch)
		b := plumbing.NewHashReference(branchRefName, plumbing.ZeroHash)

		err = w.Checkout(&git.CheckoutOptions{
			Branch: b.Name(),
			Create: true,
		})
		if err != nil {
			headRef, err := repo.Head()
			if err != nil {
				return "", err
			}
			b = plumbing.NewHashReference(branchRefName, headRef.Hash())
			err = repo.Storer.SetReference(b)
			if err != nil {
				return "", err
			}
			err = w.Checkout(&git.CheckoutOptions{Branch: b.Name()})
			if err != nil {
				return "", err
			}
		}
	}

	for _, file := range gitCommit.Spec.Files {
		var content []byte

		// Determine content source
		if file.UseRestAPIData {
			// Use REST API response data
			if gitCommit.Status.RestAPIStatus != nil && gitCommit.Status.RestAPIStatus.FormattedOutput != "" {
				content = []byte(gitCommit.Status.RestAPIStatus.FormattedOutput)
			} else {
				return "", fmt.Errorf("file %s requested REST API data but no formatted output available", file.Path)
			}
		} else {
			// Use provided content
			content = []byte(file.Content)
		}

		targetPath := file.Path

		// Encrypt content if encryption is enabled
		if encryption.ShouldEncryptFile(file.Path, gitCommit.Spec.Encryption) {
			encryptedContent, err := r.encryptFileContent(ctx, content, gitCommit.Spec.Encryption, gitCommit.Namespace)
			if err != nil {
				return "", fmt.Errorf("failed to encrypt file %s: %w", file.Path, err)
			}
			content = encryptedContent
			targetPath = encryption.GetEncryptedFilePath(file.Path, gitCommit.Spec.Encryption)
		}

		filePath := filepath.Join(tempDir, targetPath)
		dir := filepath.Dir(filePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", err
		}

		if err := ioutil.WriteFile(filePath, content, 0644); err != nil {
			return "", err
		}

		if _, err := w.Add(targetPath); err != nil {
			return "", err
		}
	}

	// Process resource references
	for _, resourceRef := range gitCommit.Spec.ResourceRefs {
		resourceFiles, err := r.processResourceRef(ctx, resourceRef, gitCommit.Namespace)
		if err != nil {
			return "", fmt.Errorf("failed to process resource reference %s/%s: %w", resourceRef.Kind, resourceRef.Name, err)
		}

		for _, file := range resourceFiles {
			targetPath := file.Path

			// Handle write modes
			var content []byte
			if resourceRef.Strategy.WriteMode == gitv1.WriteModeAppend {
				// Read existing file if it exists
				tempFilePath := filepath.Join(tempDir, file.Path)
				if existingContent, err := ioutil.ReadFile(tempFilePath); err == nil {
					content = append(existingContent, []byte("\n"+file.Content)...)
				} else {
					content = []byte(file.Content)
				}
			} else {
				// Default to overwrite
				content = []byte(file.Content)
			}

			// Encrypt content if encryption is enabled
			if encryption.ShouldEncryptFile(file.Path, gitCommit.Spec.Encryption) {
				encryptedContent, err := r.encryptFileContent(ctx, content, gitCommit.Spec.Encryption, gitCommit.Namespace)
				if err != nil {
					return "", fmt.Errorf("failed to encrypt resource file %s: %w", file.Path, err)
				}
				content = encryptedContent
				targetPath = encryption.GetEncryptedFilePath(file.Path, gitCommit.Spec.Encryption)
			}

			filePath := filepath.Join(tempDir, targetPath)
			dir := filepath.Dir(filePath)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return "", err
			}

			if err := ioutil.WriteFile(filePath, content, 0644); err != nil {
				return "", err
			}

			if _, err := w.Add(targetPath); err != nil {
				return "", err
			}
		}
	}

	commit, err := w.Commit(gitCommit.Spec.CommitMessage, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Git Change Operator",
			Email: "git-change-operator@galos.one",
			When:  time.Now(),
		},
	})
	if err != nil {
		return "", err
	}

	err = repo.Push(&git.PushOptions{
		Auth: auth,
	})
	if err != nil {
		return "", err
	}

	return commit.String(), nil
}

func (r *GitCommitReconciler) updateStatus(ctx context.Context, gitCommit *gitv1.GitCommit, phase gitv1.GitCommitPhase, message string) error {
	gitCommit.Status.Phase = phase
	gitCommit.Status.Message = message
	now := metav1.Now()
	gitCommit.Status.LastSync = &now

	return r.Status().Update(ctx, gitCommit)
}

func (r *GitCommitReconciler) fetchResource(ctx context.Context, resourceRef gitv1.ResourceRef, namespace string) (*unstructured.Unstructured, error) {
	gvk := schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    resourceRef.Kind,
	}

	if strings.Contains(resourceRef.ApiVersion, "/") {
		parts := strings.SplitN(resourceRef.ApiVersion, "/", 2)
		gvk.Group = parts[0]
		gvk.Version = parts[1]
	} else {
		gvk.Version = resourceRef.ApiVersion
	}

	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)

	nsName := types.NamespacedName{
		Name:      resourceRef.Name,
		Namespace: resourceRef.Namespace,
	}
	if resourceRef.Namespace == "" {
		nsName.Namespace = namespace
	}

	err := r.Get(ctx, nsName, obj)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch resource %s/%s: %w", resourceRef.Kind, resourceRef.Name, err)
	}

	return obj, nil
}

func (r *GitCommitReconciler) processResourceRef(ctx context.Context, resourceRef gitv1.ResourceRef, namespace string) ([]gitv1.File, error) {
	obj, err := r.fetchResource(ctx, resourceRef, namespace)
	if err != nil {
		return nil, err
	}

	var files []gitv1.File

	switch resourceRef.Strategy.Type {
	case gitv1.OutputTypeDump:
		content, err := yaml.Marshal(obj)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal resource to YAML: %w", err)
		}
		files = append(files, gitv1.File{
			Path:    resourceRef.Strategy.Path,
			Content: string(content),
		})

	case gitv1.OutputTypeFields:
		data, found, err := unstructured.NestedMap(obj.Object, "data")
		if !found || err != nil {
			return nil, fmt.Errorf("resource does not have data fields or failed to extract: %w", err)
		}

		for key, value := range data {
			fileName := fmt.Sprintf("%s/%s", strings.TrimSuffix(resourceRef.Strategy.Path, "/"), key)
			content := fmt.Sprintf("%v", value)
			files = append(files, gitv1.File{
				Path:    fileName,
				Content: content,
			})
		}

	case gitv1.OutputTypeSingleField:
		if resourceRef.Strategy.FieldRef == nil {
			return nil, fmt.Errorf("fieldRef is required for single-field strategy")
		}

		data, found, err := unstructured.NestedMap(obj.Object, "data")
		if !found || err != nil {
			return nil, fmt.Errorf("resource does not have data fields: %w", err)
		}

		value, exists := data[resourceRef.Strategy.FieldRef.Key]
		if !exists {
			return nil, fmt.Errorf("field %s not found in resource data", resourceRef.Strategy.FieldRef.Key)
		}

		var filePath string
		content := fmt.Sprintf("%v", value)

		// For append mode, write directly to the path file
		if resourceRef.Strategy.WriteMode == gitv1.WriteModeAppend {
			filePath = resourceRef.Strategy.Path
		} else {
			// For overwrite mode, create path/filename structure
			fileName := resourceRef.Strategy.FieldRef.FileName
			if fileName == "" {
				fileName = resourceRef.Strategy.FieldRef.Key
			}
			filePath = fmt.Sprintf("%s/%s", strings.TrimSuffix(resourceRef.Strategy.Path, "/"), fileName)
		}

		files = append(files, gitv1.File{
			Path:    filePath,
			Content: content,
		})

	default:
		return nil, fmt.Errorf("unsupported output strategy type: %s", resourceRef.Strategy.Type)
	}

	return files, nil
}

func (r *GitCommitReconciler) encryptFileContent(ctx context.Context, content []byte, encryptionConfig *gitv1.Encryption, namespace string) ([]byte, error) {
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

func (r *GitCommitReconciler) resolveRecipients(ctx context.Context, recipients []gitv1.Recipient, namespace string) ([]gitv1.Recipient, error) {
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

// checkRestAPICondition checks if the REST API condition is met and extracts data for use in commits
func (r *GitCommitReconciler) checkRestAPICondition(ctx context.Context, gitCommit *gitv1.GitCommit) (bool, error) {
	log := log.FromContext(ctx)
	restAPI := gitCommit.Spec.RestAPI

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
		body = bytes.NewReader([]byte(restAPI.Body))
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
		token, err := r.getTokenFromSecret(ctx, gitCommit.Namespace, restAPI.AuthSecretRef, restAPI.AuthSecretKey)
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
	if gitCommit.Status.RestAPIStatus == nil {
		gitCommit.Status.RestAPIStatus = &gitv1.RestAPIStatus{}
	}

	now := metav1.Now()
	gitCommit.Status.RestAPIStatus.LastCallTime = &now
	gitCommit.Status.RestAPIStatus.CallCount++

	if err != nil {
		// Record failed request metrics
		r.metricsCollector.RecordAPIRequest(restAPI.URL, method, "error", duration, 0)
		gitCommit.Status.RestAPIStatus.LastError = err.Error()
		log.Error(err, "REST API call failed", "url", restAPI.URL, "duration", duration)
		return false, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	gitCommit.Status.RestAPIStatus.LastStatusCode = resp.StatusCode

	// Read full response body for processing
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		// Record metrics for successful HTTP but failed body read
		r.metricsCollector.RecordAPIRequest(restAPI.URL, method, fmt.Sprintf("%d", resp.StatusCode), duration, 0)
		gitCommit.Status.RestAPIStatus.LastError = fmt.Sprintf("failed to read response: %v", err)
		return false, fmt.Errorf("failed to read response body: %w", err)
	}

	// Record successful request metrics
	r.metricsCollector.RecordAPIRequest(restAPI.URL, method, fmt.Sprintf("%d", resp.StatusCode), duration, int64(len(respBody)))

	// Store truncated response for status (max 1024 chars)
	if len(respBody) > 1024 {
		gitCommit.Status.RestAPIStatus.LastResponse = string(respBody[:1024]) + "... (truncated)"
	} else {
		gitCommit.Status.RestAPIStatus.LastResponse = string(respBody)
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
		gitCommit.Status.RestAPIStatus.ConditionMet = false
		gitCommit.Status.RestAPIStatus.LastError = fmt.Sprintf("HTTP status condition not met: %d", resp.StatusCode)
		log.Info("REST API HTTP status condition not met", "statusCode", resp.StatusCode)
		return false, nil
	}

	// Process JSON response if parsing is configured
	conditionMet := true
	if restAPI.ResponseParsing != nil {
		var err error
		conditionMet, err = r.processJSONResponse(ctx, gitCommit, respBody, restAPI.ResponseParsing)
		if err != nil {
			r.metricsCollector.RecordJSONParsingError("processing_failed")
			gitCommit.Status.RestAPIStatus.LastError = fmt.Sprintf("JSON processing failed: %v", err)
			log.Error(err, "Failed to process JSON response")
			return false, fmt.Errorf("JSON processing failed: %w", err)
		}
	}

	gitCommit.Status.RestAPIStatus.ConditionMet = conditionMet
	gitCommit.Status.RestAPIStatus.LastError = ""

	if conditionMet {
		gitCommit.Status.RestAPIStatus.SuccessCount++
		r.metricsCollector.RecordConditionCheck("success")
	} else {
		r.metricsCollector.RecordConditionCheck("json_condition_failed")
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
func (r *GitCommitReconciler) processJSONResponse(ctx context.Context, gitCommit *gitv1.GitCommit, respBody []byte, parsing *gitv1.ResponseParsing) (bool, error) {
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

		gitCommit.Status.RestAPIStatus.ExtractedData = extractedData

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

		gitCommit.Status.RestAPIStatus.FormattedOutput = strings.Join(outputParts, separator)

		log.Info("Data extracted from JSON response",
			"dataFields", parsing.DataFields,
			"extractedData", extractedData,
			"formattedOutput", gitCommit.Status.RestAPIStatus.FormattedOutput)
	}

	return true, nil
}

// getTokenFromSecret retrieves a token from a Kubernetes secret for REST API authentication
func (r *GitCommitReconciler) getTokenFromSecret(ctx context.Context, namespace, secretName, secretKey string) (string, error) {
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

func (r *GitCommitReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gitv1.GitCommit{}).
		Complete(r)
}
