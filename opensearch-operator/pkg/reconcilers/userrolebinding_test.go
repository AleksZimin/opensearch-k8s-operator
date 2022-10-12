package reconcilers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/jarcoal/httpmock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	opsterv1 "opensearch.opster.io/api/v1"
	"opensearch.opster.io/opensearch-gateway/requests"
	"opensearch.opster.io/opensearch-gateway/responses"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type TestTypes struct {
	Cluster           *opsterv1.OpenSearchCluster
	Instance          *opsterv1.OpensearchUserRoleBinding
	Reconciler        *UserRoleBindingReconciler
	Transport         *httpmock.MockTransport
	ExtraContextCalls int
}

type CustomSpec struct {
	Users        []string
	BackendRoles []string
}

type CustomStatus struct {
	ProvisionedRoles        []string
	ProvisionedUsers        []string
	ProvisionedBackendRoles []string
}

type ObjectsForSave struct {
	Users        []string
	BackendRoles []string
}

type ObjectsForDelete struct {
	Users        []string
	BackendRoles []string
}

var _ = Describe("userrolebinding reconciler", func() {
	var (
		transport  *httpmock.MockTransport
		reconciler *UserRoleBindingReconciler
		instance   *opsterv1.OpensearchUserRoleBinding
		recorder   *record.FakeRecorder

		// Objects
		ns      *corev1.Namespace
		cluster *opsterv1.OpenSearchCluster
	)

	BeforeEach(func() {
		transport = httpmock.NewMockTransport()
		transport.RegisterNoResponder(httpmock.NewNotFoundResponder(failMessage))
		instance = &opsterv1.OpensearchUserRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-role",
				Namespace: "test-urb",
				UID:       types.UID("testuid"),
			},
			Spec: opsterv1.OpensearchUserRoleBindingSpec{
				OpensearchRef: corev1.LocalObjectReference{
					Name: "test-cluster",
				},
				Users: []string{
					"test-user",
				},
				Roles: []string{
					"test-role",
				},
				BackendRoles: []string{
					"test-backend-role",
				},
			},
		}

		// Sleep for cache to start
		time.Sleep(time.Second)
		// Set up prereq-objects
		ns = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-urb",
			},
		}
		Expect(func() error {
			err := k8sClient.Get(context.Background(), client.ObjectKeyFromObject(ns), &corev1.Namespace{})
			if err != nil {
				if k8serrors.IsNotFound(err) {
					return k8sClient.Create(context.Background(), ns)
				}
				return err
			}
			return nil
		}()).To(Succeed())
		cluster = &opsterv1.OpenSearchCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster",
				Namespace: "test-urb",
			},
			Spec: opsterv1.ClusterSpec{
				General: opsterv1.GeneralConfig{
					ServiceName: "test-cluster",
				},
				NodePools: []opsterv1.NodePool{
					{
						Component: "node",
						Roles: []string{
							"master",
							"data",
						},
					},
				},
			},
		}
		Expect(func() error {
			err := k8sClient.Get(context.Background(), client.ObjectKeyFromObject(cluster), &opsterv1.OpenSearchCluster{})
			if err != nil {
				if k8serrors.IsNotFound(err) {
					return k8sClient.Create(context.Background(), cluster)
				}
				return err
			}
			return nil
		}()).To(Succeed())
	})

	JustBeforeEach(func() {
		reconciler = NewUserRoleBindingReconciler(
			context.Background(),
			k8sClient,
			recorder,
			instance,
			WithOSClientTransport(transport),
			WithUpdateStatus(false),
		)
	})

	// When("cluster doesn't exist", func() {
	// 	BeforeEach(func() {
	// 		instance.Spec.OpensearchRef.Name = "doesnotexist"
	// 		recorder = record.NewFakeRecorder(1)
	// 	})
	// 	It("should wait for the cluster to exist", func() {
	// 		go func() {
	// 			defer GinkgoRecover()
	// 			defer close(recorder.Events)
	// 			result, err := reconciler.Reconcile()
	// 			Expect(err).NotTo(HaveOccurred())
	// 			Expect(result.Requeue).To(BeTrue())
	// 		}()
	// 		var events []string
	// 		for msg := range recorder.Events {
	// 			events = append(events, msg)
	// 		}
	// 		Expect(len(events)).To(Equal(1))
	// 		Expect(events[0]).To(Equal(fmt.Sprintf("Normal %s waiting for opensearch cluster to exist", opensearchPending)))
	// 	})
	// })

	// When("cluster doesn't match status", func() {
	// 	BeforeEach(func() {
	// 		uid := types.UID("someuid")
	// 		instance.Status.ManagedCluster = &uid
	// 		recorder = record.NewFakeRecorder(1)
	// 	})
	// 	It("should error", func() {
	// 		go func() {
	// 			defer GinkgoRecover()
	// 			defer close(recorder.Events)
	// 			_, err := reconciler.Reconcile()
	// 			Expect(err).To(HaveOccurred())
	// 		}()
	// 		var events []string
	// 		for msg := range recorder.Events {
	// 			events = append(events, msg)
	// 		}
	// 		Expect(len(events)).To(Equal(1))
	// 		Expect(events[0]).To(Equal(fmt.Sprintf("Warning %s cannot change the cluster a userrolebinding refers to", opensearchRefMismatch)))
	// 	})
	// })

	// When("cluster is not ready", func() {
	// 	BeforeEach(func() {
	// 		recorder = record.NewFakeRecorder(1)
	// 	})
	// 	It("should wait for the cluster to be running", func() {
	// 		go func() {
	// 			defer GinkgoRecover()
	// 			defer close(recorder.Events)
	// 			result, err := reconciler.Reconcile()
	// 			Expect(err).NotTo(HaveOccurred())
	// 			Expect(result.Requeue).To(BeTrue())
	// 		}()
	// 		var events []string
	// 		for msg := range recorder.Events {
	// 			events = append(events, msg)
	// 		}
	// 		Expect(len(events)).To(Equal(1))
	// 		Expect(events[0]).To(Equal(fmt.Sprintf("Normal %s waiting for opensearch cluster status to be running", opensearchPending)))
	// 	})
	// })

	Context("cluster is ready", func() {
		extraContextCalls := 1
		BeforeEach(func() {
			Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(cluster), cluster)).To(Succeed())
			cluster.Status.Phase = opsterv1.PhaseRunning
			cluster.Status.ComponentsStatus = []opsterv1.ComponentStatus{}
			Expect(k8sClient.Status().Update(context.Background(), cluster)).To(Succeed())
			Eventually(func() string {
				err := k8sClient.Get(context.Background(), client.ObjectKeyFromObject(cluster), cluster)
				if err != nil {
					return "failed"
				}
				return cluster.Status.Phase
			}).Should(Equal(opsterv1.PhaseRunning))

			transport.RegisterResponder(
				http.MethodGet,
				fmt.Sprintf(
					"https://%s.%s.svc.cluster.local:9200/",
					cluster.Spec.General.ServiceName,
					cluster.Namespace,
				),
				httpmock.NewStringResponder(200, "OK").Times(2, failMessage),
			)

			transport.RegisterResponder(
				http.MethodHead,
				fmt.Sprintf(
					"https://%s.%s.svc.cluster.local:9200/",
					cluster.Spec.General.ServiceName,
					cluster.Namespace,
				),
				httpmock.NewStringResponder(200, "OK").Once(failMessage),
			)
		})

		// When("role mapping does not exist", func() {
		// 	DescribeTable("should create the role mapping",
		// 		ExecuteReconcileAction,
		// 		Entry("When users and backendRoles exist in spec",
		// 			CustomSpec{Users: []string{"test-user", "test-user-second"}, BackendRoles: []string{"test-backend-role", "test-backend-role-second"}},
		// 			requests.RoleMapping{},
		// 			404, 1, true, func() TestTypes {
		// 				return TestTypes{Cluster: cluster, Instance: instance, Reconciler: reconciler, Transport: transport, ExtraContextCalls: extraContextCalls}
		// 			}),
		// 		Entry("When users only exist in spec",
		// 			CustomSpec{Users: []string{"test-user", "test-user-second"}, BackendRoles: []string{}},
		// 			requests.RoleMapping{},
		// 			404, 1, true, func() TestTypes {
		// 				return TestTypes{Cluster: cluster, Instance: instance, Reconciler: reconciler, Transport: transport, ExtraContextCalls: extraContextCalls}
		// 			}),
		// 		Entry("When backendRoles only exist in spec",
		// 			CustomSpec{Users: []string{}, BackendRoles: []string{"test-backend-role", "test-backend-role-second"}},
		// 			requests.RoleMapping{},
		// 			404, 1, true, func() TestTypes {
		// 				return TestTypes{Cluster: cluster, Instance: instance, Reconciler: reconciler, Transport: transport, ExtraContextCalls: extraContextCalls}
		// 			}),
		// 	)
		// })

		// When("role mapping exists and contains spec's users and(or) backendRoles", func() {
		// 	DescribeTable("should do nothing",
		// 		ProcessReconcileAction,
		// 		Entry("When users and backendRoles exist in spec and in OpenSearch mapping",
		// 			CustomSpec{Users: []string{"test-user"}, BackendRoles: []string{"test-backend-role"}},
		// 			requests.RoleMapping{Users: []string{"manual-user", "test-user"}, BackendRoles: []string{"manual-backend-role", "test-backend-role"}}, CustomStatus{}, ObjectsForSave{}, ObjectsForDelete{},
		// 			200, 2, false, func() TestTypes {
		// 				return TestTypes{Cluster: cluster, Instance: instance, Reconciler: reconciler, Transport: transport, ExtraContextCalls: extraContextCalls + 1}
		// 			}),
		// 		// Entry("When users only exist in spec and in OpenSearch mapping exist users and backendRoles",
		// 		// 	CustomSpec{Users: []string{"test-user"}, BackendRoles: []string{}},
		// 		// 	requests.RoleMapping{Users: []string{"manual-user", "test-user"}, BackendRoles: []string{"manual-backend-role", "test-backend-role"}},
		// 		// 	200, 2, false, func() TestTypes {
		// 		// 		return TestTypes{Cluster: cluster, Instance: instance, Reconciler: reconciler, Transport: transport, ExtraContextCalls: extraContextCalls + 1}
		// 		// 	}),
		// 		// Entry("When users only exist in spec and in OpenSearch mapping",
		// 		// 	CustomSpec{Users: []string{"test-user"}, BackendRoles: []string{}},
		// 		// 	requests.RoleMapping{Users: []string{"manual-user", "test-user"}, BackendRoles: []string{}},
		// 		// 	200, 2, false, func() TestTypes {
		// 		// 		return TestTypes{Cluster: cluster, Instance: instance, Reconciler: reconciler, Transport: transport, ExtraContextCalls: extraContextCalls + 1}
		// 		// 	}),
		// 		// Entry("When backendRoles only exist in spec and in OpenSearch mapping exist users and backendRoles",
		// 		// 	CustomSpec{Users: []string{}, BackendRoles: []string{"test-backend-role"}},
		// 		// 	requests.RoleMapping{Users: []string{"manual-user", "test-user"}, BackendRoles: []string{"manual-backend-role", "test-backend-role"}},
		// 		// 	200, 2, false, func() TestTypes {
		// 		// 		return TestTypes{Cluster: cluster, Instance: instance, Reconciler: reconciler, Transport: transport, ExtraContextCalls: extraContextCalls + 1}
		// 		// 	}),
		// 		// Entry("When backendRoles only exist in spec and in OpenSearch mapping",
		// 		// 	CustomSpec{Users: []string{}, BackendRoles: []string{"test-backend-role"}},
		// 		// 	requests.RoleMapping{Users: []string{}, BackendRoles: []string{"manual-backend-role", "test-backend-role"}},
		// 		// 	200, 2, false, func() TestTypes {
		// 		// 		return TestTypes{Cluster: cluster, Instance: instance, Reconciler: reconciler, Transport: transport, ExtraContextCalls: extraContextCalls + 1}
		// 		// 	}),
		// 	)
		// })

		// When("role mapping exists and does not contain spec's users and(or) backendRoles", func() {
		// 	DescribeTable("should update the mapping",
		// 		ProcessReconcileAction,
		// 		Entry("When users and backendRoles exist in spec and does not exist in OpenSearch mapping. In OpenSearch mapping exist some manually created users and backendRoles",
		// 			CustomSpec{Users: []string{"test-user"}, BackendRoles: []string{"test-backend-role"}},
		// 			requests.RoleMapping{Users: []string{"manual-user"}, BackendRoles: []string{"manual-backend-role"}}, CustomStatus{}, ObjectsForSave{}, ObjectsForDelete{},
		// 			200, 2, true, func() TestTypes {
		// 				return TestTypes{Cluster: cluster, Instance: instance, Reconciler: reconciler, Transport: transport, ExtraContextCalls: extraContextCalls + 1}
		// 			}),
		// 		// Entry("When users and backendRoles exist in spec and does not exist in OpenSearch mapping. In OpenSearch mapping exist some manually created users",
		// 		// 	CustomSpec{Users: []string{"test-user"}, BackendRoles: []string{"test-backend-role"}},
		// 		// 	requests.RoleMapping{Users: []string{"manual-user"}, BackendRoles: []string{}},
		// 		// 	200, 2, true, func() TestTypes {
		// 		// 		return TestTypes{Cluster: cluster, Instance: instance, Reconciler: reconciler, Transport: transport, ExtraContextCalls: extraContextCalls + 1}
		// 		// 	}),
		// 		// Entry("When users and backendRoles exist in spec and does not exist in OpenSearch mapping. In OpenSearch mapping exist some manually created backendRoles",
		// 		// 	CustomSpec{Users: []string{"test-user"}, BackendRoles: []string{"test-backend-role"}},
		// 		// 	requests.RoleMapping{Users: []string{}, BackendRoles: []string{"manual-backend-role"}},
		// 		// 	200, 2, true, func() TestTypes {
		// 		// 		return TestTypes{Cluster: cluster, Instance: instance, Reconciler: reconciler, Transport: transport, ExtraContextCalls: extraContextCalls + 1}
		// 		// 	}),
		// 		// Entry("When users and backendRoles exist in spec and in OpenSearch mapping does not exist any users and backendRoles",
		// 		// 	CustomSpec{Users: []string{"test-user"}, BackendRoles: []string{"test-backend-role"}},
		// 		// 	requests.RoleMapping{Users: []string{}, BackendRoles: []string{}},
		// 		// 	200, 2, true, func() TestTypes {
		// 		// 		return TestTypes{Cluster: cluster, Instance: instance, Reconciler: reconciler, Transport: transport, ExtraContextCalls: extraContextCalls + 1}
		// 		// 	}),
		// 		// Entry("When users only exist in spec and does not exist in OpenSearch mapping. In OpenSearch mapping exist some manually created users and backendRoles",
		// 		// 	CustomSpec{Users: []string{"test-user"}, BackendRoles: []string{}},
		// 		// 	requests.RoleMapping{Users: []string{"manual-user"}, BackendRoles: []string{"manual-backend-role"}},
		// 		// 	200, 2, true, func() TestTypes {
		// 		// 		return TestTypes{Cluster: cluster, Instance: instance, Reconciler: reconciler, Transport: transport, ExtraContextCalls: extraContextCalls + 1}
		// 		// 	}),
		// 		// Entry("When users only exist in spec and does not exist in OpenSearch mapping. In OpenSearch mapping exist some manually created users",
		// 		// 	CustomSpec{Users: []string{"test-user"}, BackendRoles: []string{}},
		// 		// 	requests.RoleMapping{Users: []string{"manual-user"}, BackendRoles: []string{}},
		// 		// 	200, 2, true, func() TestTypes {
		// 		// 		return TestTypes{Cluster: cluster, Instance: instance, Reconciler: reconciler, Transport: transport, ExtraContextCalls: extraContextCalls + 1}
		// 		// 	}),
		// 		// Entry("When users only exist in spec and does not exist in OpenSearch mapping. In OpenSearch mapping exist some manually created backendRoles",
		// 		// 	CustomSpec{Users: []string{"test-user"}, BackendRoles: []string{}},
		// 		// 	requests.RoleMapping{Users: []string{}, BackendRoles: []string{"manual-backend-role"}},
		// 		// 	200, 2, true, func() TestTypes {
		// 		// 		return TestTypes{Cluster: cluster, Instance: instance, Reconciler: reconciler, Transport: transport, ExtraContextCalls: extraContextCalls + 1}
		// 		// 	}),
		// 		// Entry("When users only exist in spec and in OpenSearch mapping does not exist any users and backendRoles",
		// 		// 	CustomSpec{Users: []string{"test-user"}, BackendRoles: []string{}},
		// 		// 	requests.RoleMapping{Users: []string{}, BackendRoles: []string{}},
		// 		// 	200, 2, true, func() TestTypes {
		// 		// 		return TestTypes{Cluster: cluster, Instance: instance, Reconciler: reconciler, Transport: transport, ExtraContextCalls: extraContextCalls + 1}
		// 		// 	}),
		// 		// Entry("When backendRoles only exist in spec and does not exist in OpenSearch mapping. In OpenSearch mapping exist some manually created users and backendRoles",
		// 		// 	CustomSpec{Users: []string{}, BackendRoles: []string{"test-backend-role"}},
		// 		// 	requests.RoleMapping{Users: []string{"manual-user"}, BackendRoles: []string{"manual-backend-role"}},
		// 		// 	200, 2, true, func() TestTypes {
		// 		// 		return TestTypes{Cluster: cluster, Instance: instance, Reconciler: reconciler, Transport: transport, ExtraContextCalls: extraContextCalls + 1}
		// 		// 	}),
		// 		// Entry("When backendRoles only exist in spec and does not exist in OpenSearch mapping. In OpenSearch mapping exist some manually created users",
		// 		// 	CustomSpec{Users: []string{}, BackendRoles: []string{"test-backend-role"}},
		// 		// 	requests.RoleMapping{Users: []string{"manual-user"}, BackendRoles: []string{}},
		// 		// 	200, 2, true, func() TestTypes {
		// 		// 		return TestTypes{Cluster: cluster, Instance: instance, Reconciler: reconciler, Transport: transport, ExtraContextCalls: extraContextCalls + 1}
		// 		// 	}),
		// 		// Entry("When backendRoles only exist in spec and does not exist in OpenSearch mapping. In OpenSearch mapping exist some manually created backendRoles",
		// 		// 	CustomSpec{Users: []string{}, BackendRoles: []string{"test-backend-role"}},
		// 		// 	requests.RoleMapping{Users: []string{}, BackendRoles: []string{"manual-backend-role"}},
		// 		// 	200, 2, true, func() TestTypes {
		// 		// 		return TestTypes{Cluster: cluster, Instance: instance, Reconciler: reconciler, Transport: transport, ExtraContextCalls: extraContextCalls + 1}
		// 		// 	}),
		// 		// Entry("When backendRoles only exist in spec and in OpenSearch mapping does not exist any users and backendRoles",
		// 		// 	CustomSpec{Users: []string{}, BackendRoles: []string{"test-backend-role"}},
		// 		// 	requests.RoleMapping{Users: []string{}, BackendRoles: []string{}},
		// 		// 	200, 2, true, func() TestTypes {
		// 		// 		return TestTypes{Cluster: cluster, Instance: instance, Reconciler: reconciler, Transport: transport, ExtraContextCalls: extraContextCalls + 1}
		// 		// 	}),
		// 	)
		// })

		// When("user and(or) backendRole has been removed from the binding spec", func() {
		// 	DescribeTable("should remove user and(or) backendRole from the role",
		// 		ProcessReconcileAction,
		// 		Entry("When some users and some backendRoles has been removed from the binding spec. In OpenSearch mapping exist some manually created users and backendRoles",
		// 			CustomSpec{Users: []string{"test-user"}, BackendRoles: []string{"test-backend-role"}},
		// 			requests.RoleMapping{Users: []string{"test-user", "manual-user", "remove-user"}, BackendRoles: []string{"test-backend-role", "manual-backend-role", "remove-backend-role"}},
		// 			CustomStatus{ProvisionedUsers: []string{"remove-user", "test-user"}, ProvisionedBackendRoles: []string{"test-backend-role", "remove-backend-role"}, ProvisionedRoles: []string{"test-role"}},
		// 			ObjectsForSave{Users: []string{"manual-user"}, BackendRoles: []string{"manual-backend-role"}},
		// 			ObjectsForDelete{Users: []string{"remove-user"}, BackendRoles: []string{"remove-backend-role"}},
		// 			200, 2, true, func() TestTypes {
		// 				return TestTypes{Cluster: cluster, Instance: instance, Reconciler: reconciler, Transport: transport, ExtraContextCalls: extraContextCalls + 1}
		// 			}),
		// 		Entry("When some users and some backendRoles has been removed from the binding spec. In OpenSearch mapping exist manually created user only",
		// 			CustomSpec{Users: []string{"test-user"}, BackendRoles: []string{"test-backend-role"}},
		// 			requests.RoleMapping{Users: []string{"test-user", "manual-user", "remove-user"}, BackendRoles: []string{"test-backend-role", "remove-backend-role"}},
		// 			CustomStatus{ProvisionedUsers: []string{"remove-user", "test-user"}, ProvisionedBackendRoles: []string{"test-backend-role", "remove-backend-role"}, ProvisionedRoles: []string{"test-role"}},
		// 			ObjectsForSave{Users: []string{"manual-user"}, BackendRoles: []string{}},
		// 			ObjectsForDelete{Users: []string{"remove-user"}, BackendRoles: []string{"remove-backend-role"}},
		// 			200, 2, true, func() TestTypes {
		// 				return TestTypes{Cluster: cluster, Instance: instance, Reconciler: reconciler, Transport: transport, ExtraContextCalls: extraContextCalls + 1}
		// 			}),
		// 	)

		// })

		When("A role has been removed from the binding", func() {
			DescribeTable("should remove user and(or) backendRole from the role",
				ProcessReconcileAction,
				Entry("When role has been removed from the binding spec. In OpenSearch mapping exist manually created user and backendRole",
					CustomSpec{Users: []string{"test-user"}, BackendRoles: []string{"test-backend-role"}},
					requests.RoleMapping{Users: []string{"test-user", "manual-user"}, BackendRoles: []string{"test-backend-role", "manual-backend-role"}},
					CustomStatus{ProvisionedUsers: []string{"test-user"}, ProvisionedBackendRoles: []string{"test-backend-role"}, ProvisionedRoles: []string{"test-role", "remove-role"}},
					ObjectsForSave{Users: []string{"manual-user"}, BackendRoles: []string{"manual-backend-role"}},
					ObjectsForDelete{Users: []string{"test-user"}, BackendRoles: []string{"test-backend-role"}},
					200, 2, true, true, func() TestTypes {
						return TestTypes{Cluster: cluster, Instance: instance, Reconciler: reconciler, Transport: transport, ExtraContextCalls: extraContextCalls + 2}
					}),
			)
		})
		// When("User only: a role has been removed from the binding", func() {
		// 	var users []string

		// 	BeforeEach(func() {
		// 		instance.Spec.BackendRoles = []string{}
		// 		instance.Status.ProvisionedRoles = []string{
		// 			"test-role",
		// 			"another-role",
		// 		}
		// 		instance.Status.ProvisionedUsers = []string{
		// 			"test-user",
		// 		}
		// 		roleMappingRequest := requests.RoleMapping{
		// 			Users: []string{
		// 				"someother-user",
		// 				"test-user",
		// 			},
		// 		}
		// 		transport.RegisterResponder(
		// 			http.MethodGet,
		// 			fmt.Sprintf(
		// 				"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/rolesmapping/test-role",
		// 				cluster.Spec.General.ServiceName,
		// 				cluster.Namespace,
		// 			),
		// 			httpmock.NewJsonResponderOrPanic(200, responses.GetRoleMappingReponse{
		// 				"test-role": roleMappingRequest,
		// 			}).Times(2, failMessage),
		// 		)
		// 		transport.RegisterResponder(
		// 			http.MethodGet,
		// 			fmt.Sprintf(
		// 				"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/rolesmapping/another-role",
		// 				cluster.Spec.General.ServiceName,
		// 				cluster.Namespace,
		// 			),
		// 			httpmock.NewJsonResponderOrPanic(200, responses.GetRoleMappingReponse{
		// 				"another-role": roleMappingRequest,
		// 			}).Times(2, failMessage),
		// 		)
		// 		transport.RegisterResponder(
		// 			http.MethodPut,
		// 			fmt.Sprintf(
		// 				"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/rolesmapping/another-role",
		// 				cluster.Spec.General.ServiceName,
		// 				cluster.Namespace,
		// 			),
		// 			func(req *http.Request) (*http.Response, error) {
		// 				mapping := &requests.RoleMapping{}
		// 				if err := json.NewDecoder(req.Body).Decode(&mapping); err != nil {
		// 					return httpmock.NewStringResponse(501, ""), nil
		// 				}
		// 				users = mapping.Users
		// 				return httpmock.NewStringResponse(200, ""), nil
		// 			},
		// 		)
		// 	})

		// 	It("should remove the user from the removed role", func() {
		// 		_, err := reconciler.Reconcile()
		// 		Expect(err).NotTo(HaveOccurred())
		// 		Expect(users).To(ContainElement("someother-user"))
		// 		Expect(users).NotTo(ContainElement("test-user"))
		// 		// Confirm all responders have been called
		// 		Expect(transport.GetTotalCallCount()).To(Equal(transport.NumResponders() + extraContextCalls + 2))
		// 	})
		// })
	})

	// Context("deletions", func() {
	// 	When("cluster does not exist", func() {
	// 		BeforeEach(func() {
	// 			instance.Spec.OpensearchRef.Name = "doesnotexist"
	// 		})
	// 		It("should do nothing and exit", func() {
	// 			Expect(reconciler.Delete()).To(Succeed())
	// 		})
	// 	})
	// 	Context("checking mappings", func() {
	// 		extraContextCalls := 1
	// 		BeforeEach(func() {
	// 			transport.RegisterResponder(
	// 				http.MethodGet,
	// 				fmt.Sprintf(
	// 					"https://%s.%s.svc.cluster.local:9200/",
	// 					cluster.Spec.General.ServiceName,
	// 					cluster.Namespace,
	// 				),
	// 				httpmock.NewStringResponder(200, "OK").Times(2, failMessage),
	// 			)
	// 			transport.RegisterResponder(
	// 				http.MethodHead,
	// 				fmt.Sprintf(
	// 					"https://%s.%s.svc.cluster.local:9200/",
	// 					cluster.Spec.General.ServiceName,
	// 					cluster.Namespace,
	// 				),
	// 				httpmock.NewStringResponder(200, "OK").Once(failMessage),
	// 			)
	// 		})

	// 		When("role mapping does not exist", func() {
	// 			BeforeEach(func() {
	// 				instance.Status.ProvisionedRoles = []string{
	// 					"test-role",
	// 				}
	// 				transport.RegisterResponder(
	// 					http.MethodGet,
	// 					fmt.Sprintf(
	// 						"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/rolesmapping/test-role",
	// 						cluster.Spec.General.ServiceName,
	// 						cluster.Namespace,
	// 					),
	// 					httpmock.NewStringResponder(404, "does not exist").Once(failMessage),
	// 				)
	// 			})
	// 			It("should do nothing and exit", func() {
	// 				Expect(reconciler.Delete()).To(Succeed())
	// 				Expect(transport.GetTotalCallCount()).To(Equal(transport.NumResponders() + extraContextCalls))
	// 			})
	// 		})

	// 		When("user is only user in role mapping", func() {
	// 			BeforeEach(func() {
	// 				instance.Status.ProvisionedUsers = []string{
	// 					"test-user",
	// 				}
	// 				instance.Status.ProvisionedRoles = []string{
	// 					"test-role",
	// 				}
	// 				roleMappingRequest := requests.RoleMapping{
	// 					Users: []string{
	// 						"test-user",
	// 					},
	// 				}
	// 				transport.RegisterResponder(
	// 					http.MethodGet,
	// 					fmt.Sprintf(
	// 						"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/rolesmapping/test-role",
	// 						cluster.Spec.General.ServiceName,
	// 						cluster.Namespace,
	// 					),
	// 					httpmock.NewJsonResponderOrPanic(200, responses.GetRoleMappingReponse{
	// 						"test-role": roleMappingRequest,
	// 					}).Times(2, failMessage),
	// 				)
	// 				transport.RegisterResponder(
	// 					http.MethodDelete,
	// 					fmt.Sprintf(
	// 						"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/rolesmapping/test-role",
	// 						cluster.Spec.General.ServiceName,
	// 						cluster.Namespace,
	// 					),
	// 					httpmock.NewStringResponder(200, "OK").Once(failMessage),
	// 				)
	// 			})
	// 			It("should delete the role mapping", func() {
	// 				Expect(reconciler.Delete()).To(Succeed())
	// 				Expect(transport.GetTotalCallCount()).To(Equal(transport.NumResponders() + extraContextCalls + 1))
	// 			})
	// 		})

	// 		When("user is one of the users in the mapping", func() {
	// 			var users []string

	// 			BeforeEach(func() {
	// 				instance.Status.ProvisionedRoles = []string{
	// 					"test-role",
	// 				}
	// 				instance.Status.ProvisionedUsers = []string{
	// 					"test-user",
	// 				}
	// 				roleMappingRequest := requests.RoleMapping{
	// 					Users: []string{
	// 						"someother-user",
	// 						"test-user",
	// 					},
	// 				}
	// 				transport.RegisterResponder(
	// 					http.MethodGet,
	// 					fmt.Sprintf(
	// 						"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/rolesmapping/test-role",
	// 						cluster.Spec.General.ServiceName,
	// 						cluster.Namespace,
	// 					),
	// 					httpmock.NewJsonResponderOrPanic(200, responses.GetRoleMappingReponse{
	// 						"test-role": roleMappingRequest,
	// 					}).Times(2, failMessage),
	// 				)
	// 				transport.RegisterResponder(
	// 					http.MethodPut,
	// 					fmt.Sprintf(
	// 						"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/rolesmapping/test-role",
	// 						cluster.Spec.General.ServiceName,
	// 						cluster.Namespace,
	// 					),
	// 					func(req *http.Request) (*http.Response, error) {
	// 						mapping := &requests.RoleMapping{}
	// 						if err := json.NewDecoder(req.Body).Decode(&mapping); err != nil {
	// 							return httpmock.NewStringResponse(501, ""), nil
	// 						}
	// 						users = mapping.Users
	// 						return httpmock.NewStringResponse(200, ""), nil
	// 					},
	// 				)
	// 			})
	// 			It("should remove the user and update the mapping", func() {
	// 				Expect(reconciler.Delete()).To(Succeed())
	// 				Expect(transport.GetTotalCallCount()).To(Equal(transport.NumResponders() + extraContextCalls + 1))
	// 				Expect(users).To(ContainElement("someother-user"))
	// 				Expect(users).NotTo(ContainElement("test-user"))
	// 			})
	// 		})
	// 	})
	// })
})

func ProcessReconcileAction(customSpec CustomSpec, roleMappingRequest requests.RoleMapping, customStatus CustomStatus, objectsForSave ObjectsForSave, objectsForDelete ObjectsForDelete,
	getHttpCode int, getCount int, registerHttpPutMethod, deleteRole bool, testFunc func() TestTypes) {

	var getHttpMock httpmock.Responder
	var users []string
	var backendRoles []string
	var err error

	instance := testFunc().Instance
	reconciler := testFunc().Reconciler
	transport := testFunc().Transport
	cluster := testFunc().Cluster
	extraContextCalls := testFunc().ExtraContextCalls
	instance.Spec.Users = customSpec.Users
	instance.Spec.BackendRoles = customSpec.BackendRoles
	instance.Status.ProvisionedRoles = customStatus.ProvisionedRoles
	instance.Status.ProvisionedUsers = customStatus.ProvisionedUsers
	instance.Status.ProvisionedBackendRoles = customStatus.ProvisionedBackendRoles
	usersForSave := objectsForSave.Users
	backendRolesForSave := objectsForSave.BackendRoles
	usersForDelete := objectsForDelete.Users
	backendRolesForDelete := objectsForDelete.BackendRoles

	apiUrl := fmt.Sprintf(
		"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/rolesmapping/test-role",
		cluster.Spec.General.ServiceName,
		cluster.Namespace,
	)

	if getHttpCode == 404 {
		getHttpMock = httpmock.NewStringResponder(404, "does not exist").Once(failMessage)
	} else {
		getHttpMock = httpmock.NewJsonResponderOrPanic(200, responses.GetRoleMappingReponse{
			"test-role": roleMappingRequest,
		}).Times(getCount, failMessage)
	}

	if deleteRole == true {
		users, backendRoles, err = ExecuteReconcileActionForDeleteRole(cluster, instance, reconciler, transport, roleMappingRequest, getCount)
	} else {
		users, backendRoles, err = ExecuteReconcileAction(cluster, instance, reconciler, transport, apiUrl, getHttpMock, getHttpCode, registerHttpPutMethod)
	}

	Expect(err).NotTo(HaveOccurred())
	if registerHttpPutMethod == true {
		if deleteRole != true {
			Expect(users).To(ContainElements(instance.Spec.Users))
			Expect(backendRoles).To(ContainElements(instance.Spec.BackendRoles))
		}

		Expect(users).To(ContainElements(usersForSave))
		Expect(backendRoles).To(ContainElements(backendRolesForSave))

		if usersForDelete != nil {
			Expect(users).NotTo(ContainElements(usersForDelete))
		}
		if backendRolesForDelete != nil {
			Expect(backendRoles).NotTo(ContainElements(backendRolesForDelete))
		}
	}
	// Confirm all responders have been called
	Expect(transport.GetTotalCallCount()).To(Equal(transport.NumResponders() + extraContextCalls))

}

func ExecuteReconcileAction(cluster *opsterv1.OpenSearchCluster, instance *opsterv1.OpensearchUserRoleBinding,
	reconciler *UserRoleBindingReconciler, transport *httpmock.MockTransport,
	apiUrl string, getHttpMock httpmock.Responder, getHttpCode int, registerHttpPutMethod bool) ([]string, []string, error) {

	var users []string
	var backendRoles []string

	transport.RegisterResponder(
		http.MethodGet,
		apiUrl,
		getHttpMock,
	)

	if registerHttpPutMethod == true {
		transport.RegisterResponder(
			http.MethodPut,
			apiUrl,
			func(req *http.Request) (*http.Response, error) {
				mapping := &requests.RoleMapping{}
				if err := json.NewDecoder(req.Body).Decode(&mapping); err != nil {
					return httpmock.NewStringResponse(501, ""), nil
				}
				users = mapping.Users
				backendRoles = mapping.BackendRoles
				return httpmock.NewStringResponse(200, ""), nil
			},
		)
	}
	_, err := reconciler.Reconcile()
	fmt.Printf("FROM DescribeTable: Create users:%s, bakendRoles:%s, Spec: %s\n", users, backendRoles, instance.Spec)
	return users, backendRoles, err
}

func ExecuteReconcileActionForDeleteRole(cluster *opsterv1.OpenSearchCluster, instance *opsterv1.OpensearchUserRoleBinding,
	reconciler *UserRoleBindingReconciler, transport *httpmock.MockTransport,
	roleMappingRequest requests.RoleMapping, getCount int) ([]string, []string, error) {

	var users []string
	var backendRoles []string

	transport.RegisterResponder(
		http.MethodGet,
		fmt.Sprintf(
			"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/rolesmapping/test-role",
			cluster.Spec.General.ServiceName,
			cluster.Namespace,
		),
		httpmock.NewJsonResponderOrPanic(200, responses.GetRoleMappingReponse{
			"test-role": roleMappingRequest,
		}).Times(getCount, failMessage),
	)

	transport.RegisterResponder(
		http.MethodGet,
		fmt.Sprintf(
			"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/rolesmapping/remove-role",
			cluster.Spec.General.ServiceName,
			cluster.Namespace,
		),
		httpmock.NewJsonResponderOrPanic(200, responses.GetRoleMappingReponse{
			"remove-role": roleMappingRequest,
		}).Times(getCount, failMessage),
	)

	transport.RegisterResponder(
		http.MethodPut,
		fmt.Sprintf(
			"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/rolesmapping/remove-role",
			cluster.Spec.General.ServiceName,
			cluster.Namespace,
		),
		func(req *http.Request) (*http.Response, error) {
			mapping := &requests.RoleMapping{}
			if err := json.NewDecoder(req.Body).Decode(&mapping); err != nil {
				return httpmock.NewStringResponse(501, ""), nil
			}
			users = mapping.Users
			backendRoles = mapping.BackendRoles
			return httpmock.NewStringResponse(200, ""), nil
		},
	)
	_, err := reconciler.Reconcile()
	fmt.Printf("FROM DescribeTable: Create users:%s, bakendRoles:%s, Spec: %s\n", users, backendRoles, instance.Spec)
	return users, backendRoles, err
}
