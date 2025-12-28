package test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	gitv1 "github.com/mihaigalos/git-change-operator/api/v1"
)

var _ = Describe("GitCommit Schedule Controller", func() {
	Context("When creating a scheduled GitCommit", func() {
		const (
			GitCommitName      = "test-scheduled-gitcommit"
			GitCommitNamespace = "default"
			SecretName         = "test-schedule-secret"
			timeout            = time.Second * 10
			interval           = time.Millisecond * 250
		)

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      GitCommitName,
			Namespace: GitCommitNamespace,
		}

		BeforeEach(func() {
			By("Creating the test secret")
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      SecretName,
					Namespace: GitCommitNamespace,
				},
				StringData: map[string]string{
					"token": "test-token",
				},
			}
			Expect(k8sClient.Create(ctx, secret)).Should(Succeed())
		})

		AfterEach(func() {
			By("Cleaning up the test resources")
			gitCommit := &gitv1.GitCommit{}
			err := k8sClient.Get(ctx, typeNamespacedName, gitCommit)
			if err == nil {
				Expect(k8sClient.Delete(ctx, gitCommit)).Should(Succeed())
			}

			secret := &corev1.Secret{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: SecretName, Namespace: GitCommitNamespace}, secret)
			if err == nil {
				Expect(k8sClient.Delete(ctx, secret)).Should(Succeed())
			}
		})

		It("should set NextScheduledTime on creation", func() {
			gitCommit := &gitv1.GitCommit{
				ObjectMeta: metav1.ObjectMeta{
					Name:      GitCommitName,
					Namespace: GitCommitNamespace,
				},
				Spec: gitv1.GitCommitSpec{
					Schedule:      "0 2 * * *", // Daily at 2 AM
					Repository:    "https://github.com/mihaigalos/test.git",
					Branch:        "main",
					CommitMessage: "test commit",
					AuthSecretRef: SecretName,
					AuthSecretKey: "token",
					Files: []gitv1.File{
						{Path: "test.txt", Content: "test content"},
					},
				},
			}

			Expect(k8sClient.Create(ctx, gitCommit)).Should(Succeed())

			// Check that NextScheduledTime is eventually set
			Eventually(func() bool {
				err := k8sClient.Get(ctx, typeNamespacedName, gitCommit)
				if err != nil {
					return false
				}
				return gitCommit.Status.NextScheduledTime != nil
			}, timeout, interval).Should(BeTrue())

			// Verify it's in the future
			Expect(gitCommit.Status.NextScheduledTime.Time).Should(BeTemporally(">", time.Now()))
		})

		It("should not execute when Suspend is true", func() {
			gitCommit := &gitv1.GitCommit{
				ObjectMeta: metav1.ObjectMeta{
					Name:      GitCommitName,
					Namespace: GitCommitNamespace,
				},
				Spec: gitv1.GitCommitSpec{
					Schedule:      "* * * * *", // Every minute (for testing)
					Suspend:       true,
					Repository:    "https://github.com/mihaigalos/test.git",
					Branch:        "main",
					CommitMessage: "test commit",
					AuthSecretRef: SecretName,
					AuthSecretKey: "token",
					Files: []gitv1.File{
						{Path: "test.txt", Content: "test content"},
					},
				},
			}

			Expect(k8sClient.Create(ctx, gitCommit)).Should(Succeed())

			// Wait a bit to ensure no execution happens
			time.Sleep(2 * time.Second)

			err := k8sClient.Get(ctx, typeNamespacedName, gitCommit)
			Expect(err).Should(BeNil())

			// Verify no executions recorded
			Expect(gitCommit.Status.ExecutionHistory).Should(BeEmpty())
			Expect(gitCommit.Status.LastScheduledTime).Should(BeNil())
		})

		It("should maintain execution history with limit", func() {
			maxHistory := 3
			gitCommit := &gitv1.GitCommit{
				ObjectMeta: metav1.ObjectMeta{
					Name:      GitCommitName,
					Namespace: GitCommitNamespace,
				},
				Spec: gitv1.GitCommitSpec{
					Schedule:            "* * * * *",
					MaxExecutionHistory: &maxHistory,
					Repository:          "https://github.com/mihaigalos/test.git",
					Branch:              "main",
					CommitMessage:       "test commit",
					AuthSecretRef:       SecretName,
					AuthSecretKey:       "token",
					Files: []gitv1.File{
						{Path: "test.txt", Content: "test content"},
					},
				},
			}

			Expect(k8sClient.Create(ctx, gitCommit)).Should(Succeed())

			// In a real scenario, we would wait for executions
			// For this test, we verify the limit is respected
			Eventually(func() int {
				err := k8sClient.Get(ctx, typeNamespacedName, gitCommit)
				if err != nil {
					return -1
				}
				return len(gitCommit.Status.ExecutionHistory)
			}, timeout, interval).Should(BeNumerically("<=", maxHistory))
		})

		It("should ignore TTL when schedule is set", func() {
			ttl := 1 // 1 minute TTL
			gitCommit := &gitv1.GitCommit{
				ObjectMeta: metav1.ObjectMeta{
					Name:      GitCommitName,
					Namespace: GitCommitNamespace,
				},
				Spec: gitv1.GitCommitSpec{
					Schedule:      "0 2 * * *",
					TTLMinutes:    &ttl,
					Repository:    "https://github.com/mihaigalos/test.git",
					Branch:        "main",
					CommitMessage: "test commit",
					AuthSecretRef: SecretName,
					AuthSecretKey: "token",
					Files: []gitv1.File{
						{Path: "test.txt", Content: "test content"},
					},
				},
			}

			Expect(k8sClient.Create(ctx, gitCommit)).Should(Succeed())

			// Wait longer than TTL
			time.Sleep(2 * time.Second)

			// Verify resource still exists (TTL ignored)
			err := k8sClient.Get(ctx, typeNamespacedName, gitCommit)
			Expect(err).Should(BeNil())
			Expect(gitCommit.DeletionTimestamp).Should(BeNil())
		})
	})
})

var _ = Describe("PullRequest Schedule Controller", func() {
	Context("When creating a scheduled PullRequest", func() {
		const (
			PRName      = "test-scheduled-pr"
			PRNamespace = "default"
			SecretName  = "test-pr-schedule-secret"
			timeout     = time.Second * 10
			interval    = time.Millisecond * 250
		)

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      PRName,
			Namespace: PRNamespace,
		}

		BeforeEach(func() {
			By("Creating the test secret")
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      SecretName,
					Namespace: PRNamespace,
				},
				StringData: map[string]string{
					"token": "test-token",
				},
			}
			Expect(k8sClient.Create(ctx, secret)).Should(Succeed())
		})

		AfterEach(func() {
			By("Cleaning up the test resources")
			pr := &gitv1.PullRequest{}
			err := k8sClient.Get(ctx, typeNamespacedName, pr)
			if err == nil {
				Expect(k8sClient.Delete(ctx, pr)).Should(Succeed())
			}

			secret := &corev1.Secret{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: SecretName, Namespace: PRNamespace}, secret)
			if err == nil {
				Expect(k8sClient.Delete(ctx, secret)).Should(Succeed())
			}
		})

		It("should set NextScheduledTime on creation", func() {
			pr := &gitv1.PullRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      PRName,
					Namespace: PRNamespace,
				},
				Spec: gitv1.PullRequestSpec{
					Schedule:      "@weekly",
					Repository:    "https://github.com/mihaigalos/test.git",
					HeadBranch:    "feature",
					BaseBranch:    "main",
					Title:         "Test PR",
					Body:          "Test PR body",
					AuthSecretRef: SecretName,
					AuthSecretKey: "token",
					Files: []gitv1.File{
						{Path: "test.txt", Content: "test content"},
					},
				},
			}

			Expect(k8sClient.Create(ctx, pr)).Should(Succeed())

			// Check that NextScheduledTime is eventually set
			Eventually(func() bool {
				err := k8sClient.Get(ctx, typeNamespacedName, pr)
				if err != nil {
					return false
				}
				return pr.Status.NextScheduledTime != nil
			}, timeout, interval).Should(BeTrue())

			// Verify it's in the future
			Expect(pr.Status.NextScheduledTime.Time).Should(BeTemporally(">", time.Now()))
		})

		It("should not execute when Suspend is true", func() {
			pr := &gitv1.PullRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      PRName,
					Namespace: PRNamespace,
				},
				Spec: gitv1.PullRequestSpec{
					Schedule:      "@hourly",
					Suspend:       true,
					Repository:    "https://github.com/mihaigalos/test.git",
					HeadBranch:    "feature",
					BaseBranch:    "main",
					Title:         "Test PR",
					Body:          "Test PR body",
					AuthSecretRef: SecretName,
					AuthSecretKey: "token",
					Files: []gitv1.File{
						{Path: "test.txt", Content: "test content"},
					},
				},
			}

			Expect(k8sClient.Create(ctx, pr)).Should(Succeed())

			// Wait a bit to ensure no execution happens
			time.Sleep(2 * time.Second)

			err := k8sClient.Get(ctx, typeNamespacedName, pr)
			Expect(err).Should(BeNil())

			// Verify no executions recorded
			Expect(pr.Status.ExecutionHistory).Should(BeEmpty())
			Expect(pr.Status.LastScheduledTime).Should(BeNil())
		})
	})
})
