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
	"github.com/google/go-github/v55/github"
	"golang.org/x/oauth2"
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

type PullRequestReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=git.galos.one,resources=pullrequests,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=git.galos.one,resources=pullrequests/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=git.galos.one,resources=pullrequests/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

func (r *PullRequestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

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

func (r *PullRequestReconciler) getAuthFromSecret(ctx context.Context, namespace, secretName, secretKey string) (*http.BasicAuth, string, error) {
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

	auth := &http.BasicAuth{
		Username: username,
		Password: string(token),
	}

	return auth, string(token), nil
}

func (r *PullRequestReconciler) createPullRequest(ctx context.Context, pr *gitv1.PullRequest, auth *http.BasicAuth, token string) (int, string, error) {
	tempDir, err := ioutil.TempDir("", "pull-request-")
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

	for _, file := range pr.Spec.Files {
		filePath := filepath.Join(tempDir, file.Path)
		dir := filepath.Dir(filePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return 0, "", err
		}

		if err := ioutil.WriteFile(filePath, []byte(file.Content), 0644); err != nil {
			return 0, "", err
		}

		if _, err := w.Add(file.Path); err != nil {
			return 0, "", err
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
		return 0, "", err
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
		return 0, "", err
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

func (r *PullRequestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gitv1.PullRequest{}).
		Complete(r)
}
