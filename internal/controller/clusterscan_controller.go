/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"errors"
	"fmt"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	v1alpha1 "github.com/sakthisrivivek/cluster-scan-monitor/api/v1alpha1"
)

// ClusterScanReconciler reconciles a ClusterScan object
type ClusterScanReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=batch.clusterscan.io,resources=clusterscans,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch.clusterscan.io,resources=clusterscans/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=batch.clusterscan.io,resources=clusterscans/finalizers,verbs=update
//+kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch,resources=cronjobs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=batch,resources=cronjobs/finalizers,verbs=update
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch,resources=jobs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=batch,resources=jobs/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ClusterScan object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.3/pkg/reconcile
func (r *ClusterScanReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	var clusterScan *v1alpha1.ClusterScan
	nn := req.NamespacedName

	if err := r.Get(ctx, nn, clusterScan); err != nil {
		if apierrors.IsNotFound(err) {
			log.Log.Info("ClusterScan not found", "NN", nn)

			return ctrl.Result{}, nil
		}

		log.Log.Error(err, "Unable to fetch ClusterScan", "NN", nn)

		return ctrl.Result{}, err
	}

	log.Log.Info("Found ClusterScan", "NN", nn)

	secrets := &corev1.SecretList{}
	selector, _ := metav1.LabelSelectorAsSelector(clusterScan.Spec.Selector)

	if err := r.List(ctx, secrets,
		client.InNamespace(req.Namespace), client.MatchingLabelsSelector{Selector: selector}); err != nil {
		if !apierrors.IsNotFound(err) {
			log.Log.Error(err, "Unable to list secrets", "NN", nn, "selector", selector)

			return ctrl.Result{}, err
		}

		log.Log.Info("Found no Secrets matching selector", "NN", nn)
	}

	jobConditions := []v1alpha1.ClusterScanCondition{}

	for _, secret := range secrets.Items {
		if err := isSecretValid(secret); err != nil {
			log.Log.Error(err, "Secret is invalid", "NN", nn,
				"SecretName", secret.Name)

			continue // skip Job creating if secret is invalid
		}

		jobName := getJobName(clusterScan, secret)
		appendSecret(clusterScan, secret)

		//TODO: implement Schedule validation
		if len(clusterScan.Spec.Schedule) > 0 {
			cronJob := &batchv1.CronJob{}

			if err := r.Get(ctx, nn, cronJob); err != nil {
				if apierrors.IsNotFound(err) {
					log.Log.Info("CronJob not found. Creating...", "NN", nn)
					if err := r.createCronJob(ctx, clusterScan, secret); err != nil {
						log.Log.Error(err, "Failed to create CronJob", "NN", nn)

						return ctrl.Result{}, err
					}

					return ctrl.Result{}, nil
				}

				log.Log.Error(err, "Unable to fetch CronJob", "NN", nn, "JOB_NAME", jobName)

				return ctrl.Result{}, err
			}

			if err := r.updateCronJobSpec(ctx, clusterScan, cronJob); err != nil {
				log.Log.Error(err, "Failed to update CronJob", "NN", nn, "JOB_NAME", jobName)

				return ctrl.Result{}, err
			}

			conditions, err := r.addCronJobCondition(ctx, clusterScan.Status.Conditions, cronJob, secret)
			if err != nil {
				log.Log.Error(err, "Failed to update CronJob", "NN", nn, "JOB_NAME", jobName)
			} else {
				clusterScan.Status.Conditions = conditions
			}

		} else {
			job := &batchv1.Job{}

			if err := r.Get(ctx, nn, job); err != nil {
				if apierrors.IsNotFound(err) {
					log.Log.Info("Job not found. Creating...", "NN", nn)
					if err := r.createJob(ctx, clusterScan, secret); err != nil {
						log.Log.Error(err, "Failed to create Job", "NN", nn)

						return ctrl.Result{}, err
					}
				}

				log.Log.Error(err, "Unable to fetch Job", "NN", nn, "JOB_NAME", jobName)

				return ctrl.Result{}, err
			}

			if err := r.updateJobSpec(ctx, clusterScan, job); err != nil {
				log.Log.Error(err, "Failed to update Job", "NN", nn, "JOB_NAME", jobName)

				return ctrl.Result{}, err
			}

			clusterScan.Status.Conditions = addJobCondition(clusterScan.Status.Conditions, job, secret)
		}
	}

	clusterScan.Status.Conditions = jobConditions

	err := r.Status().Update(ctx, clusterScan)
	if err != nil {
		log.Log.Info("Failed to update Status", "NN", nn)

		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func appendSecret(clusterScan *v1alpha1.ClusterScan, secret corev1.Secret) {
	requiredEnvVars := map[string]string{
		"CLUSTER_URL": "CLUSTER_URL",
		"SECRET_KEY":  "SECRET_KEY",
	}

	// TODO: inject Secret to specific container, not only first. Take container name from Secret
	container := &clusterScan.Spec.JobTemplate.Spec.Template.Spec.Containers[0]

	envMap := make(map[string]bool)
	for _, env := range container.Env {
		if env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil {
			envMap[env.Name] = true
		}
	}

	// Append missing environment variables
	for envName, secretKey := range requiredEnvVars {
		if !envMap[envName] {
			container.Env = append(
				container.Env,
				corev1.EnvVar{
					Name: envName,
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: secret.Name,
							},
							Key: secretKey,
						},
					},
				},
			)
		}
	}
}

func isSecretValid(secret corev1.Secret) error {
	requiredKeys := []string{"CLUSTER_URL", "SECRET_KEY"}
	for _, key := range requiredKeys {
		if _, exists := secret.Data[key]; !exists {
			return errors.New("secret does not contain required key: " + key)
		}
	}
	return nil
}

func getJobName(clusterScan *v1alpha1.ClusterScan, secret corev1.Secret) string {
	return fmt.Sprintf("%s-%s", clusterScan.Name, secret.Name)
}

func (r *ClusterScanReconciler) createJob(ctx context.Context,
	clusterScan *v1alpha1.ClusterScan, secret corev1.Secret) error {

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: clusterScan.Namespace,
			Name:      getJobName(clusterScan, secret),
		},
		Spec: batchv1.JobSpec{
			Template: clusterScan.Spec.JobTemplate.Spec.Template,
		},
	}

	if err := ctrl.SetControllerReference(clusterScan, job, r.Scheme); err != nil {
		return err
	}

	return r.Create(ctx, job)
}

func (r *ClusterScanReconciler) createCronJob(ctx context.Context,
	clusterScan *v1alpha1.ClusterScan, secret corev1.Secret) error {

	job := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: clusterScan.Namespace,
			Name:      getJobName(clusterScan, secret),
		},
		Spec: batchv1.CronJobSpec{
			Schedule:    clusterScan.Spec.Schedule,
			JobTemplate: clusterScan.Spec.JobTemplate,
		},
	}

	if err := ctrl.SetControllerReference(clusterScan, job, r.Scheme); err != nil {
		return err
	}

	return r.Create(ctx, job)
}

func (r *ClusterScanReconciler) updateCronJobSpec(ctx context.Context, clusterScan *v1alpha1.ClusterScan, cronJob *batchv1.CronJob) error {
	if cronJob.Spec.Schedule != clusterScan.Spec.Schedule || !isJobTemplateSpecEqual(cronJob.Spec.JobTemplate, clusterScan.Spec.JobTemplate) {
		cronJob.Spec.Schedule = clusterScan.Spec.Schedule
		cronJob.Spec.JobTemplate = clusterScan.Spec.JobTemplate

		if err := r.Update(ctx, cronJob); err != nil {
			return err
		}
	}

	return nil
}

// isJobTemplateEqual checks if two JobTemplateSpecs are equal
func isJobTemplateSpecEqual(template1, template2 batchv1.JobTemplateSpec) bool {
	return reflect.DeepEqual(template1, template2)
}

func (r *ClusterScanReconciler) updateJobSpec(ctx context.Context, clusterScan *v1alpha1.ClusterScan, job *batchv1.Job) error {
	if !isJobTemplateEqual(job.Spec.Template, clusterScan.Spec.JobTemplate.Spec.Template) {
		job.Spec.Template = clusterScan.Spec.JobTemplate.Spec.Template

		if err := r.Update(ctx, job); err != nil {
			return err
		}
	}

	return nil
}

func isJobTemplateEqual(template1, template2 corev1.PodTemplateSpec) bool {
	return reflect.DeepEqual(template1, template2)
}

func addJobCondition(jobConditions []v1alpha1.ClusterScanCondition, job *batchv1.Job,
	secret corev1.Secret) []v1alpha1.ClusterScanCondition {

	var jobStatus v1alpha1.ClusterScanConditionStatus

	for _, condition := range job.Status.Conditions {
		if condition.Type == batchv1.JobComplete && condition.Status == corev1.ConditionTrue {
			jobStatus = v1alpha1.ConditionSucceeded
			break
		} else if condition.Type == batchv1.JobFailed && condition.Status == corev1.ConditionTrue {
			jobStatus = v1alpha1.ConditionFailed
			break
		}
	}

	newCondition := v1alpha1.ClusterScanCondition{
		ClusterName:       secret.Name,
		Status:            jobStatus,
		LastExecutionTime: *job.Status.CompletionTime,
	}

	jobConditions = append(jobConditions, newCondition)
	return jobConditions
}

func (r *ClusterScanReconciler) addCronJobCondition(ctx context.Context, jobConditions []v1alpha1.ClusterScanCondition,
	cronJob *batchv1.CronJob, secret corev1.Secret) ([]v1alpha1.ClusterScanCondition, error) {

	latestJob, err := r.getLatestJobFromCronJob(ctx, cronJob)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest job from CronJob: %v", err)
	}

	var jobStatus v1alpha1.ClusterScanConditionStatus
	for _, condition := range latestJob.Status.Conditions {
		if condition.Type == batchv1.JobComplete && condition.Status == corev1.ConditionTrue {
			jobStatus = v1alpha1.ConditionSucceeded
			break
		} else if condition.Type == batchv1.JobFailed && condition.Status == corev1.ConditionTrue {
			jobStatus = v1alpha1.ConditionFailed
			break
		}
	}

	newCondition := v1alpha1.ClusterScanCondition{
		ClusterName:       secret.Name,
		Status:            jobStatus,
		LastExecutionTime: *latestJob.Status.CompletionTime,
	}

	// Append the new condition to the existing list
	jobConditions = append(jobConditions, newCondition)

	return jobConditions, nil
}

func (r *ClusterScanReconciler) getLatestJobFromCronJob(ctx context.Context,
	cronJob *batchv1.CronJob) (*batchv1.Job, error) {
	jobList := &batchv1.JobList{}

	listOptions := []client.ListOption{
		client.InNamespace(cronJob.Namespace),
		client.MatchingLabels(cronJob.Spec.JobTemplate.ObjectMeta.Labels),
	}

	if err := r.List(ctx, jobList, listOptions...); err != nil {
		if !apierrors.IsNotFound(err) {
			log.Log.Error(err, "Failed to list Jobs for CronJob", "Name", cronJob.Name,
				"Namespace", cronJob.Namespace)

			return nil, err
		}

		log.Log.Error(err, "Found no Jobs for CronJob", "Name", cronJob.Name,
			"Namespace", cronJob.Namespace)

		return nil, err
	}

	var latestJob *batchv1.Job
	for _, job := range jobList.Items {
		ownerReferences := job.GetOwnerReferences()
		for _, owner := range ownerReferences {
			if owner.UID == cronJob.UID {
				if latestJob == nil || job.CreationTimestamp.Time.After(latestJob.CreationTimestamp.Time) {
					latestJob = &job
				}
				break
			}
		}
	}

	if latestJob == nil {
		return nil, fmt.Errorf("no job found owned by CronJob %s/%s", cronJob.Namespace, cronJob.Name)
	}

	return latestJob, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterScanReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ClusterScan{}).
		Owns(&batchv1.CronJob{}).
		Owns(&batchv1.Job{}).
		Owns(&corev1.Secret{}).
		Complete(r)
}
