package controllers

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	gitv1 "github.com/mihaigalos/git-change-operator/api/v1"
)

type GitCommitReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=git.galos.one,resources=gitcommits,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=git.galos.one,resources=gitcommits/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=git.galos.one,resources=gitcommits/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

func (r *GitCommitReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

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

func (r *GitCommitReconciler) getAuthFromSecret(ctx context.Context, namespace, secretName, secretKey string) (*http.BasicAuth, error) {
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

	return &http.BasicAuth{
		Username: username,
		Password: string(token),
	}, nil
}

func (r *GitCommitReconciler) performGitCommit(ctx context.Context, gitCommit *gitv1.GitCommit, auth *http.BasicAuth) (string, error) {
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
		filePath := filepath.Join(tempDir, file.Path)
		dir := filepath.Dir(filePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", err
		}

		if err := ioutil.WriteFile(filePath, []byte(file.Content), 0644); err != nil {
			return "", err
		}

		if _, err := w.Add(file.Path); err != nil {
			return "", err
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

func (r *GitCommitReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gitv1.GitCommit{}).
		Complete(r)
}
