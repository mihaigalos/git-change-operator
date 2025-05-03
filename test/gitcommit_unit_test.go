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

func TestGitCommitReconciler_Reconcile(t *testing.T) {
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
		gitCommit     *gitv1.GitCommit
		secret        *corev1.Secret
		expectedPhase gitv1.GitCommitPhase
		expectError   bool
	}{
		{
			name: "missing secret should fail",
			gitCommit: &gitv1.GitCommit{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-commit",
					Namespace: "default",
				},
				Spec: gitv1.GitCommitSpec{
					Repository:    "https://github.com/test/repo.git",
					Branch:        "main",
					CommitMessage: "Test commit",
					AuthSecretRef: "missing-secret",
					Files: []gitv1.File{
						{Path: "test.txt", Content: "test"},
					},
				},
			},
			expectedPhase: gitv1.GitCommitPhaseFailed,
			expectError:   false,
		},
		{
			name: "valid gitcommit with secret should process",
			gitCommit: &gitv1.GitCommit{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-commit",
					Namespace: "default",
				},
				Spec: gitv1.GitCommitSpec{
					Repository:    "https://github.com/test/repo.git",
					Branch:        "main",
					CommitMessage: "Test commit",
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
					"token": []byte("fake-token"),
				},
			},
			expectedPhase: gitv1.GitCommitPhaseFailed,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objects := []runtime.Object{tt.gitCommit}
			if tt.secret != nil {
				objects = append(objects, tt.secret)
			}

			client := fake.NewClientBuilder().
				WithScheme(scheme).
				WithRuntimeObjects(objects...).
				WithStatusSubresource(&gitv1.GitCommit{}).
				Build()

			reconciler := &controllers.GitCommitReconciler{
				Client: client,
				Scheme: scheme,
			}

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      tt.gitCommit.ObjectMeta.Name,
					Namespace: tt.gitCommit.ObjectMeta.Namespace,
				},
			}

			_, err := reconciler.Reconcile(context.TODO(), req)
			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			updatedGitCommit := &gitv1.GitCommit{}
			err = client.Get(context.TODO(), req.NamespacedName, updatedGitCommit)
			if err != nil {
				t.Errorf("Failed to get updated GitCommit: %v", err)
			}

			if updatedGitCommit.Status.Phase != tt.expectedPhase {
				t.Errorf("Expected phase %v, got %v", tt.expectedPhase, updatedGitCommit.Status.Phase)
			}
		})
	}
}

func TestGitCommitValidation(t *testing.T) {
	tests := []struct {
		name      string
		gitCommit gitv1.GitCommit
		isValid   bool
	}{
		{
			name: "valid gitcommit",
			gitCommit: gitv1.GitCommit{
				Spec: gitv1.GitCommitSpec{
					Repository:    "https://github.com/test/repo.git",
					Branch:        "main",
					CommitMessage: "Test commit",
					AuthSecretRef: "test-secret",
					Files: []gitv1.File{
						{Path: "test.txt", Content: "test"},
					},
				},
			},
			isValid: true,
		},
		{
			name: "missing repository",
			gitCommit: gitv1.GitCommit{
				Spec: gitv1.GitCommitSpec{
					Branch:        "main",
					CommitMessage: "Test commit",
					AuthSecretRef: "test-secret",
					Files: []gitv1.File{
						{Path: "test.txt", Content: "test"},
					},
				},
			},
			isValid: false,
		},
		{
			name: "empty files",
			gitCommit: gitv1.GitCommit{
				Spec: gitv1.GitCommitSpec{
					Repository:    "https://github.com/test/repo.git",
					Branch:        "main",
					CommitMessage: "Test commit",
					AuthSecretRef: "test-secret",
					Files:         []gitv1.File{},
				},
			},
			isValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := validateGitCommit(tt.gitCommit)
			if isValid != tt.isValid {
				t.Errorf("Expected validation result %v, got %v", tt.isValid, isValid)
			}
		})
	}
}

func validateGitCommit(gc gitv1.GitCommit) bool {
	if gc.Spec.Repository == "" {
		return false
	}
	if gc.Spec.CommitMessage == "" {
		return false
	}
	if gc.Spec.AuthSecretRef == "" {
		return false
	}
	if len(gc.Spec.Files) == 0 {
		return false
	}
	for _, file := range gc.Spec.Files {
		if file.Path == "" || file.Content == "" {
			return false
		}
	}
	return true
}
