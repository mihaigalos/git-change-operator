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
	"github.com/mihaigalos/git-change-operator/pkg/cel"
	"github.com/mihaigalos/git-change-operator/pkg/encryption"
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

	// Check if the resource has expired due to TTL
	expired, err := r.checkTTLExpired(ctx, &gitCommit)
	if err != nil {
		log.Error(err, "failed to check TTL expiration")
		return ctrl.Result{}, err
	}
	if expired {
		log.Info("Deleting expired GitCommit resource")
		if err := r.Delete(ctx, &gitCommit); err != nil {
			log.Error(err, "failed to delete expired GitCommit")
			return ctrl.Result{RequeueAfter: time.Minute * 1}, err
		}
		return ctrl.Result{}, nil
	}

	// For committed resources, still requeue periodically for TTL checking
	if gitCommit.Status.Phase == gitv1.GitCommitPhaseCommitted {
		return ctrl.Result{RequeueAfter: time.Minute * 1}, nil
	}

	// For failed resources, only check TTL - don't retry the operation
	// But still requeue periodically for TTL checking
	if gitCommit.Status.Phase == gitv1.GitCommitPhaseFailed {
		return ctrl.Result{RequeueAfter: time.Minute * 1}, nil
	}

	if err := r.updateStatus(ctx, &gitCommit, gitv1.GitCommitPhaseRunning, "Processing git commit"); err != nil {
		return ctrl.Result{}, err
	}

	// Check REST API conditions if configured
	if len(gitCommit.Spec.RestAPIs) > 0 {
		allConditionsMet, err := r.checkRestAPIConditions(ctx, &gitCommit)
		if err != nil {
			log.Error(err, "failed to check REST API conditions")
			r.updateStatus(ctx, &gitCommit, gitv1.GitCommitPhaseFailed, fmt.Sprintf("REST API check failed: %v", err))
			return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
		}

		if !allConditionsMet {
			log.Info("One or more REST API conditions not met, skipping git commit")
			r.updateStatus(ctx, &gitCommit, gitv1.GitCommitPhasePending, "REST API conditions not met, waiting...")
			return ctrl.Result{RequeueAfter: time.Minute * 2}, nil
		}

		log.Info("All REST API conditions met, proceeding with git commit")
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
			// Use REST API response data from multiple APIs
			content = r.buildFileContent(&file, gitCommit.Status.RestAPIStatuses)
			if len(content) == 0 {
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

// checkRestAPIConditions checks if all REST API conditions are met and extracts data for use in commits
func (r *GitCommitReconciler) checkRestAPIConditions(ctx context.Context, gitCommit *gitv1.GitCommit) (bool, error) {
	log := log.FromContext(ctx)

	// Initialize status slice if needed
	if gitCommit.Status.RestAPIStatuses == nil {
		gitCommit.Status.RestAPIStatuses = make([]gitv1.RestAPIStatus, len(gitCommit.Spec.RestAPIs))
	}

	// Ensure we have the right number of status entries
	if len(gitCommit.Status.RestAPIStatuses) != len(gitCommit.Spec.RestAPIs) {
		gitCommit.Status.RestAPIStatuses = make([]gitv1.RestAPIStatus, len(gitCommit.Spec.RestAPIs))
	}

	allConditionsMet := true

	// Process each REST API
	for i, restAPI := range gitCommit.Spec.RestAPIs {
		conditionMet, err := r.checkSingleRestAPICondition(ctx, gitCommit, &restAPI, &gitCommit.Status.RestAPIStatuses[i])
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
func (r *GitCommitReconciler) checkSingleRestAPICondition(ctx context.Context, gitCommit *gitv1.GitCommit, restAPI *gitv1.RestAPI, status *gitv1.RestAPIStatus) (bool, error) {
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
func (r *GitCommitReconciler) processJSONResponse(ctx context.Context, status *gitv1.RestAPIStatus, respBody []byte, parsing *gitv1.ResponseParsing) (bool, error) {
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

// checkTTLExpired checks if the resource has expired based on TTL configuration
func (r *GitCommitReconciler) checkTTLExpired(ctx context.Context, gitCommit *gitv1.GitCommit) (bool, error) {
	log := log.FromContext(ctx)

	// If TTLMinutes is not set, no TTL expiration
	if gitCommit.Spec.TTLMinutes == nil {
		return false, nil
	}

	// Calculate expiration time
	creationTime := gitCommit.CreationTimestamp.Time
	ttlDuration := time.Duration(*gitCommit.Spec.TTLMinutes) * time.Minute
	expirationTime := creationTime.Add(ttlDuration)

	// Check if expired
	now := time.Now()
	if now.After(expirationTime) {
		log.Info("GitCommit resource has expired due to TTL",
			"creationTime", creationTime,
			"ttlMinutes", *gitCommit.Spec.TTLMinutes,
			"expirationTime", expirationTime,
			"currentTime", now)
		return true, nil
	}

	log.V(1).Info("GitCommit resource TTL check",
		"creationTime", creationTime,
		"ttlMinutes", *gitCommit.Spec.TTLMinutes,
		"expirationTime", expirationTime,
		"timeToExpiration", expirationTime.Sub(now))
	return false, nil
}

// buildFileContent builds the content for a file based on REST API data and configuration
func (r *GitCommitReconciler) buildFileContent(file *gitv1.File, statuses []gitv1.RestAPIStatus) []byte {
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

func (r *GitCommitReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gitv1.GitCommit{}).
		Complete(r)
}
