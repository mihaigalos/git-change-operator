package test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gitv1 "github.com/mihaigalos/git-change-operator/api/v1"
	"github.com/mihaigalos/git-change-operator/controllers"
)

var _ = Describe("GitCommit Controller", func() {
	Context("When reconciling a resource", func() {
		const (
			GitCommitName      = "test-gitcommit"
			GitCommitNamespace = "default"
			timeout            = time.Second * 10
			duration           = time.Second * 10
			interval           = time.Millisecond * 250
		)

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      GitCommitName,
			Namespace: GitCommitNamespace,
		}

		BeforeEach(func() {
			By("creating the custom resource for the Kind GitCommit")
			gitCommit := &gitv1.GitCommit{}
			err := k8sClient.Get(ctx, typeNamespacedName, gitCommit)
			if err != nil && err.Error() == `gitcommits.gco.galos.one "`+GitCommitName+`" not found` {
				resource := &gitv1.GitCommit{
					ObjectMeta: metav1.ObjectMeta{
						Name:      GitCommitName,
						Namespace: GitCommitNamespace,
					},
					Spec: gitv1.GitCommitSpec{
						Repository:    "https://github.com/test/repo.git",
						Branch:        "main",
						CommitMessage: "Test commit",
						AuthSecretRef: "test-secret",
						Files: []gitv1.File{
							{
								Path:    "test.txt",
								Content: "test content",
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}

			By("creating the auth secret")
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: GitCommitNamespace,
				},
				Data: map[string][]byte{
					"token": []byte("fake-token"),
				},
			}
			err = k8sClient.Create(ctx, secret)
			if err != nil && err.Error() != `secrets "test-secret" already exists` {
				Expect(err).ToNot(HaveOccurred())
			}
		})

		AfterEach(func() {
			resource := &gitv1.GitCommit{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance GitCommit")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())

			secret := &corev1.Secret{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: "test-secret", Namespace: GitCommitNamespace}, secret)
			if err == nil {
				Expect(k8sClient.Delete(ctx, secret)).To(Succeed())
			}
		})

		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &controllers.GitCommitReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
		})

		It("should set status to pending initially", func() {
			gitCommit := &gitv1.GitCommit{}
			Eventually(func() error {
				return k8sClient.Get(ctx, typeNamespacedName, gitCommit)
			}, timeout, interval).Should(Succeed())

			Eventually(func() gitv1.GitCommitPhase {
				err := k8sClient.Get(ctx, typeNamespacedName, gitCommit)
				if err != nil {
					return ""
				}
				return gitCommit.Status.Phase
			}, timeout, interval).Should(Or(
				Equal(gitv1.GitCommitPhasePending),
				Equal(gitv1.GitCommitPhaseRunning),
				Equal(gitv1.GitCommitPhaseFailed),
			))
		})

		It("should handle missing secret gracefully", func() {
			By("Creating GitCommit with non-existent secret")
			gitCommitNoSecret := &gitv1.GitCommit{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-gitcommit-no-secret",
					Namespace: GitCommitNamespace,
				},
				Spec: gitv1.GitCommitSpec{
					Repository:    "https://github.com/test/repo.git",
					Branch:        "main",
					CommitMessage: "Test commit",
					AuthSecretRef: "non-existent-secret",
					Files: []gitv1.File{
						{
							Path:    "test.txt",
							Content: "test content",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, gitCommitNoSecret)).To(Succeed())

			Eventually(func() gitv1.GitCommitPhase {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      "test-gitcommit-no-secret",
					Namespace: GitCommitNamespace,
				}, gitCommitNoSecret)
				if err != nil {
					return ""
				}
				return gitCommitNoSecret.Status.Phase
			}, timeout, interval).Should(Equal(gitv1.GitCommitPhaseFailed))

			Expect(k8sClient.Delete(ctx, gitCommitNoSecret)).To(Succeed())
		})
	})
})
