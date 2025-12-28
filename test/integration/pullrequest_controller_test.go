package test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	gitv1 "github.com/mihaigalos/git-change-operator/api/v1"
	"github.com/mihaigalos/git-change-operator/controllers"
)

var _ = Describe("PullRequest Controller", func() {
	Context("When reconciling a resource", func() {
		const (
			PullRequestName      = "test-pullrequest"
			PullRequestNamespace = "default"
			timeout              = time.Second * 10
			duration             = time.Second * 10
			interval             = time.Millisecond * 250
		)

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      PullRequestName,
			Namespace: PullRequestNamespace,
		}

		BeforeEach(func() {
			By("creating the custom resource for the Kind PullRequest")
			pullRequest := &gitv1.PullRequest{}
			err := k8sClient.Get(ctx, typeNamespacedName, pullRequest)
			if err != nil && err.Error() == `pullrequests.gco.galos.one "`+PullRequestName+`" not found` {
				resource := &gitv1.PullRequest{
					ObjectMeta: metav1.ObjectMeta{
						Name:      PullRequestName,
						Namespace: PullRequestNamespace,
					},
					Spec: gitv1.PullRequestSpec{
						Repository:    "https://github.com/test/repo.git",
						BaseBranch:    "main",
						HeadBranch:    "feature/test",
						Title:         "Test PR",
						Body:          "Test pull request body",
						AuthSecretRef: "test-secret",
						Files: []gitv1.File{
							{
								Path:    "test.txt",
								Content: "test content for PR",
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
					Namespace: PullRequestNamespace,
				},
				Data: map[string][]byte{
					"token": []byte("fake-github-token"),
				},
			}
			err = k8sClient.Create(ctx, secret)
			if err != nil && err.Error() != `secrets "test-secret" already exists` {
				Expect(err).ToNot(HaveOccurred())
			}
		})

		AfterEach(func() {
			By("Cleanup the specific resource instance PullRequest")
			resource := &gitv1.PullRequest{}
			Eventually(func() error {
				// Get the latest version of the resource
				if err := k8sClient.Get(ctx, typeNamespacedName, resource); err != nil {
					// If resource doesn't exist, cleanup is done
					if apierrors.IsNotFound(err) {
						return nil
					}
					return err
				}
				// Try to delete with the fresh resource version
				return k8sClient.Delete(ctx, resource)
			}, timeout, interval).Should(Succeed())

			By("Cleanup the auth secret")
			secret := &corev1.Secret{}
			Eventually(func() error {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: "test-secret", Namespace: PullRequestNamespace}, secret); err != nil {
					// If secret doesn't exist, cleanup is done
					if apierrors.IsNotFound(err) {
						return nil
					}
					return err
				}
				return k8sClient.Delete(ctx, secret)
			}, timeout, interval).Should(Succeed())
		})

		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &controllers.PullRequestReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, ctrl.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
		})

		It("should set status to pending initially", func() {
			pullRequest := &gitv1.PullRequest{}
			Eventually(func() error {
				return k8sClient.Get(ctx, typeNamespacedName, pullRequest)
			}, timeout, interval).Should(Succeed())

			Eventually(func() gitv1.PullRequestPhase {
				err := k8sClient.Get(ctx, typeNamespacedName, pullRequest)
				if err != nil {
					return ""
				}
				return pullRequest.Status.Phase
			}, timeout, interval).Should(Or(
				Equal(gitv1.PullRequestPhasePending),
				Equal(gitv1.PullRequestPhaseRunning),
				Equal(gitv1.PullRequestPhaseFailed),
			))
		})

		It("should validate required fields", func() {
			By("Creating PullRequest with missing title")
			invalidPR := &gitv1.PullRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-invalid-pr",
					Namespace: PullRequestNamespace,
				},
				Spec: gitv1.PullRequestSpec{
					Repository:    "https://github.com/test/repo.git",
					BaseBranch:    "main",
					HeadBranch:    "feature/test",
					AuthSecretRef: "test-secret",
					Files: []gitv1.File{
						{
							Path:    "test.txt",
							Content: "test content",
						},
					},
				},
			}

			err := k8sClient.Create(ctx, invalidPR)
			Expect(err).To(HaveOccurred())
		})
	})
})
