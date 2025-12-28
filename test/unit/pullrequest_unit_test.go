package test

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gitv1 "github.com/mihaigalos/git-change-operator/api/v1"
	"github.com/mihaigalos/git-change-operator/controllers"
	corev1 "k8s.io/api/core/v1"
)

func TestPullRequestReconciler_Reconcile(t *testing.T) {
	scheme := runtime.NewScheme()
	err := gitv1.AddToScheme(scheme)
	if err != nil {
		t.Fatalf("Failed to add scheme: %v", err)
	}
	err = corev1.AddToScheme(scheme)
	if err != nil {
		t.Fatalf("Failed to add core scheme: %v", err)
	}

	tests := []struct {
		name          string
		pullRequest   *gitv1.PullRequest
		secret        *corev1.Secret
		expectedPhase gitv1.PullRequestPhase
		expectError   bool
	}{
		{
			name: "missing secret should fail",
			pullRequest: &gitv1.PullRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pr",
					Namespace: "default",
				},
				Spec: gitv1.PullRequestSpec{
					Repository:    "https://github.com/test/repo.git",
					BaseBranch:    "main",
					HeadBranch:    "feature/test",
					Title:         "Test PR",
					AuthSecretRef: "missing-secret",
					Files: []gitv1.File{
						{Path: "test.txt", Content: "test"},
					},
				},
			},
			expectedPhase: gitv1.PullRequestPhaseFailed,
			expectError:   false,
		},
		{
			name: "valid pullrequest with secret should process",
			pullRequest: &gitv1.PullRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pr",
					Namespace: "default",
				},
				Spec: gitv1.PullRequestSpec{
					Repository:    "https://github.com/test/repo.git",
					BaseBranch:    "main",
					HeadBranch:    "feature/test",
					Title:         "Test PR",
					AuthSecretRef: "test-secret",
					Files: []gitv1.File{
						{Path: "test.txt", Content: "test"},
					},
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"token": []byte("fake-github-token"),
				},
			},
			expectedPhase: gitv1.PullRequestPhaseFailed,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objects := []runtime.Object{tt.pullRequest}
			if tt.secret != nil {
				objects = append(objects, tt.secret)
			}

			client := fake.NewClientBuilder().
				WithScheme(scheme).
				WithRuntimeObjects(objects...).
				WithStatusSubresource(&gitv1.PullRequest{}).
				Build()

			reconciler := &controllers.PullRequestReconciler{
				Client: client,
				Scheme: scheme,
			}

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      tt.pullRequest.ObjectMeta.Name,
					Namespace: tt.pullRequest.ObjectMeta.Namespace,
				},
			}

			_, err := reconciler.Reconcile(context.TODO(), req)
			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			updatedPR := &gitv1.PullRequest{}
			err = client.Get(context.TODO(), req.NamespacedName, updatedPR)
			if err != nil {
				t.Errorf("Failed to get updated PullRequest: %v", err)
			}

			if updatedPR.Status.Phase != tt.expectedPhase {
				t.Errorf("Expected phase %v, got %v", tt.expectedPhase, updatedPR.Status.Phase)
			}
		})
	}
}

func TestPullRequestValidation(t *testing.T) {
	tests := []struct {
		name        string
		pullRequest gitv1.PullRequest
		isValid     bool
	}{
		{
			name: "valid pullrequest",
			pullRequest: gitv1.PullRequest{
				Spec: gitv1.PullRequestSpec{
					Repository:    "https://github.com/test/repo.git",
					BaseBranch:    "main",
					HeadBranch:    "feature/test",
					Title:         "Test PR",
					AuthSecretRef: "test-secret",
					Files: []gitv1.File{
						{Path: "test.txt", Content: "test"},
					},
				},
			},
			isValid: true,
		},
		{
			name: "missing title",
			pullRequest: gitv1.PullRequest{
				Spec: gitv1.PullRequestSpec{
					Repository:    "https://github.com/test/repo.git",
					BaseBranch:    "main",
					HeadBranch:    "feature/test",
					AuthSecretRef: "test-secret",
					Files: []gitv1.File{
						{Path: "test.txt", Content: "test"},
					},
				},
			},
			isValid: false,
		},
		{
			name: "same base and head branch",
			pullRequest: gitv1.PullRequest{
				Spec: gitv1.PullRequestSpec{
					Repository:    "https://github.com/test/repo.git",
					BaseBranch:    "main",
					HeadBranch:    "main",
					Title:         "Test PR",
					AuthSecretRef: "test-secret",
					Files: []gitv1.File{
						{Path: "test.txt", Content: "test"},
					},
				},
			},
			isValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := validatePullRequest(tt.pullRequest)
			if isValid != tt.isValid {
				t.Errorf("Expected validation result %v, got %v", tt.isValid, isValid)
			}
		})
	}
}

func validatePullRequest(pr gitv1.PullRequest) bool {
	if pr.Spec.Repository == "" {
		return false
	}
	if pr.Spec.Title == "" {
		return false
	}
	if pr.Spec.BaseBranch == "" || pr.Spec.HeadBranch == "" {
		return false
	}
	if pr.Spec.BaseBranch == pr.Spec.HeadBranch {
		return false
	}
	if pr.Spec.AuthSecretRef == "" {
		return false
	}
	if len(pr.Spec.Files) == 0 {
		return false
	}
	for _, file := range pr.Spec.Files {
		if file.Path == "" || file.Content == "" {
			return false
		}
	}
	return true
}
