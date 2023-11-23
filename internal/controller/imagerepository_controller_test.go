/*
Copyright 2022 The Flux authors

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
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/v1/remote"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"

	imagev1 "github.com/fluxcd/image-reflector-controller/api/v1beta2"
	"github.com/fluxcd/image-reflector-controller/internal/registry"
	"github.com/fluxcd/image-reflector-controller/internal/test"
)

// mockDatabase mocks the image repository database.
type mockDatabase struct {
	TagData    []string
	ReadError  error
	WriteError error
}

// SetTags implements the DatabaseWriter interface of the Database.
func (db *mockDatabase) SetTags(repo string, tags []string) error {
	if db.WriteError != nil {
		return db.WriteError
	}
	db.TagData = append(db.TagData, tags...)
	return nil
}

// Tags implements the DatabaseReader interface of the Database.
func (db mockDatabase) Tags(repo string) ([]string, error) {
	if db.ReadError != nil {
		return nil, db.ReadError
	}
	return db.TagData, nil
}

func TestImageRepositoryReconciler_deleteBeforeFinalizer(t *testing.T) {
	g := NewWithT(t)

	namespaceName := "imagerepo-" + randStringRunes(5)
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: namespaceName},
	}
	g.Expect(k8sClient.Create(ctx, namespace)).ToNot(HaveOccurred())
	t.Cleanup(func() {
		g.Expect(k8sClient.Delete(ctx, namespace)).NotTo(HaveOccurred())
	})

	imagerepo := &imagev1.ImageRepository{}
	imagerepo.Name = "test-gitrepo"
	imagerepo.Namespace = namespaceName
	imagerepo.Spec = imagev1.ImageRepositorySpec{
		Interval: metav1.Duration{Duration: interval},
		Image:    "test-image",
	}
	// Add a test finalizer to prevent the object from getting deleted.
	imagerepo.SetFinalizers([]string{"test-finalizer"})
	g.Expect(k8sClient.Create(ctx, imagerepo)).NotTo(HaveOccurred())
	// Add deletion timestamp by deleting the object.
	g.Expect(k8sClient.Delete(ctx, imagerepo)).NotTo(HaveOccurred())

	r := &ImageRepositoryReconciler{
		Client:        k8sClient,
		EventRecorder: record.NewFakeRecorder(32),
	}
	// NOTE: Only a real API server responds with an error in this scenario.
	_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(imagerepo)})
	g.Expect(err).NotTo(HaveOccurred())
}

func TestImageRepositoryReconciler_shouldScan(t *testing.T) {
	testImage := "example.com/foo/bar"
	tests := []struct {
		name          string
		beforeFunc    func(obj *imagev1.ImageRepository, reconcileTime time.Time)
		db            *mockDatabase
		reconcileTime time.Time
		wantErr       bool
		wantScan      bool
		wantNextScan  time.Duration
		wantReason    string
	}{
		{
			name:         "new object",
			wantScan:     true,
			wantNextScan: time.Minute,
			wantReason:   scanReasonNeverScanned,
		},
		{
			name: "first reconcile at annotation",
			beforeFunc: func(obj *imagev1.ImageRepository, reconcileTime time.Time) {
				obj.SetAnnotations(map[string]string{meta.ReconcileRequestAnnotation: "now"})
				obj.Status.LastScanResult = &imagev1.ScanResult{
					ScanTime: metav1.Time{Time: reconcileTime.Add(-time.Second * 30)},
				}
			},
			wantScan:     true,
			wantNextScan: time.Minute,
			wantReason:   scanReasonReconcileRequested,
		},
		{
			name: "second reconcile at annotation",
			beforeFunc: func(obj *imagev1.ImageRepository, reconcileTime time.Time) {
				obj.SetAnnotations(map[string]string{meta.ReconcileRequestAnnotation: "now"})
				obj.Status.LastHandledReconcileAt = "foo"
				obj.Status.LastScanResult = &imagev1.ScanResult{
					ScanTime: metav1.Time{Time: reconcileTime.Add(-time.Second * 30)},
				}
			},
			wantScan:     true,
			wantNextScan: time.Minute,
			wantReason:   scanReasonReconcileRequested,
		},
		{
			name:          "reconcile at annotation with same value",
			reconcileTime: time.Now(),
			beforeFunc: func(obj *imagev1.ImageRepository, reconcileTime time.Time) {
				obj.SetAnnotations(map[string]string{meta.ReconcileRequestAnnotation: "now"})
				obj.Status.CanonicalImageName = testImage
				obj.Status.LastHandledReconcileAt = "now"
				obj.Status.LastScanResult = &imagev1.ScanResult{
					ScanTime: metav1.NewTime(reconcileTime.Add(-time.Second * 30)),
				}
			},
			db:           &mockDatabase{TagData: []string{"foo"}},
			wantScan:     false,
			wantNextScan: time.Second * 30,
		},
		{
			name:          "change image",
			reconcileTime: time.Now(),
			beforeFunc: func(obj *imagev1.ImageRepository, reconcileTime time.Time) {
				obj.Status.CanonicalImageName = testImage
				newImage := "example.com/other/image"
				obj.Spec.Image = newImage
				obj.Status.LastScanResult = &imagev1.ScanResult{
					ScanTime: metav1.NewTime(reconcileTime.Add(-time.Second * 30)),
				}
			},
			db:           &mockDatabase{TagData: []string{"foo"}},
			wantScan:     true,
			wantNextScan: time.Minute,
			wantReason:   scanReasonNewImageName,
		},
		{
			name:          "exclusion list change",
			reconcileTime: time.Now(),
			beforeFunc: func(obj *imagev1.ImageRepository, reconcileTime time.Time) {
				obj.Status.ObservedExclusionList = []string{"baz"}
				obj.Spec.ExclusionList = []string{"bar"}
				obj.Status.CanonicalImageName = testImage
				obj.Status.LastScanResult = &imagev1.ScanResult{
					ScanTime: metav1.NewTime(reconcileTime.Add(-time.Second * 30)),
				}
			},
			db:           &mockDatabase{TagData: []string{"foo"}},
			wantScan:     true,
			wantNextScan: time.Minute,
			wantReason:   scanReasonUpdatedExclusionList,
		},
		{
			name:          "no tags",
			reconcileTime: time.Now(),
			beforeFunc: func(obj *imagev1.ImageRepository, reconcileTime time.Time) {
				obj.Status.CanonicalImageName = testImage
				obj.Status.LastScanResult = &imagev1.ScanResult{
					TagCount: 0,
					ScanTime: metav1.NewTime(reconcileTime.Add(-time.Second * 10)),
				}
			},
			db:           &mockDatabase{},
			wantScan:     true,
			wantNextScan: time.Minute,
			wantReason:   scanReasonEmptyDatabase,
		},
		{
			name:          "database read failure",
			reconcileTime: time.Now(),
			beforeFunc: func(obj *imagev1.ImageRepository, reconcileTime time.Time) {
				obj.Status.CanonicalImageName = testImage
				obj.Status.LastScanResult = &imagev1.ScanResult{
					ScanTime: metav1.NewTime(reconcileTime.Add(-time.Second * 30)),
				}
			},
			db:           &mockDatabase{TagData: []string{"foo"}, ReadError: errors.New("fail")},
			wantErr:      true,
			wantScan:     false,
			wantNextScan: time.Minute,
		},
		{
			name:          "after the interval",
			reconcileTime: time.Now(),
			beforeFunc: func(obj *imagev1.ImageRepository, reconcileTime time.Time) {
				obj.Status.CanonicalImageName = testImage
				obj.Status.LastScanResult = &imagev1.ScanResult{
					ScanTime: metav1.NewTime(reconcileTime.Add(-time.Minute * 2)),
				}
			},
			db:           &mockDatabase{TagData: []string{"foo"}},
			wantScan:     true,
			wantNextScan: time.Minute,
			wantReason:   scanReasonInterval,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			r := &ImageRepositoryReconciler{
				EventRecorder: record.NewFakeRecorder(32),
				Database:      tt.db,
				patchOptions:  getPatchOptions(imageRepositoryOwnedConditions, "irc"),
			}

			obj := &imagev1.ImageRepository{}
			obj.Spec.Image = testImage
			obj.Spec.Interval = metav1.Duration{Duration: time.Minute}
			obj.Spec.ExclusionList = []string{"aaa"}
			obj.Status.ObservedExclusionList = []string{"aaa"}

			if tt.beforeFunc != nil {
				tt.beforeFunc(obj, tt.reconcileTime)
			}

			scan, next, scanReason, err := r.shouldScan(*obj, tt.reconcileTime)
			g.Expect(err != nil).To(Equal(tt.wantErr))
			g.Expect(scan).To(Equal(tt.wantScan))
			g.Expect(next).To(Equal(tt.wantNextScan))
			g.Expect(scanReason).To(Equal(tt.wantReason))
		})
	}
}

func TestImageRepositoryReconciler_scan(t *testing.T) {
	registryServer := test.NewRegistryServer()
	defer registryServer.Close()

	tests := []struct {
		name           string
		tags           []string
		exclusionList  []string
		annotation     string
		db             *mockDatabase
		wantErr        bool
		wantTags       []string
		wantLatestTags []string
	}{
		{
			name:    "no tags",
			wantErr: true,
		},
		{
			name:           "simple tags",
			tags:           []string{"a", "b", "c", "d"},
			db:             &mockDatabase{},
			wantTags:       []string{"a", "b", "c", "d"},
			wantLatestTags: []string{"d", "c", "b", "a"},
		},
		{
			name:           "simple tags, 10+",
			tags:           []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"},
			db:             &mockDatabase{},
			wantTags:       []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"},
			wantLatestTags: []string{"k", "j", "i", "h, g, f, e, d, c, b"},
		},
		{
			name:           "with single exclusion pattern",
			tags:           []string{"a", "b", "c", "d"},
			exclusionList:  []string{"c"},
			db:             &mockDatabase{},
			wantTags:       []string{"a", "b", "d"},
			wantLatestTags: []string{"d", "b", "a"},
		},
		{
			name:           "with multiple exclusion pattern",
			tags:           []string{"a", "b", "c", "d"},
			exclusionList:  []string{"c", "a"},
			db:             &mockDatabase{},
			wantTags:       []string{"b", "d"},
			wantLatestTags: []string{"d", "b"},
		},
		{
			name:          "bad exclusion pattern",
			tags:          []string{"a"}, // Ensure repo isn't empty to prevent 404.
			exclusionList: []string{"[="},
			wantErr:       true,
		},
		{
			name:    "db write fails",
			tags:    []string{"a", "b"},
			db:      &mockDatabase{WriteError: errors.New("fail")},
			wantErr: true,
		},
		{
			name:           "with reconcile annotation",
			tags:           []string{"a", "b"},
			annotation:     "foo",
			db:             &mockDatabase{},
			wantTags:       []string{"a", "b"},
			wantLatestTags: []string{"b", "a"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			imgRepo, _, err := test.LoadImages(registryServer, "test-fetch-"+randStringRunes(5), tt.tags)
			g.Expect(err).ToNot(HaveOccurred())

			r := ImageRepositoryReconciler{
				EventRecorder: record.NewFakeRecorder(32),
				Database:      tt.db,
				patchOptions:  getPatchOptions(imageRepositoryOwnedConditions, "irc"),
			}

			repo := &imagev1.ImageRepository{}
			repo.Spec = imagev1.ImageRepositorySpec{
				Image:         imgRepo,
				ExclusionList: tt.exclusionList,
			}

			if tt.annotation != "" {
				repo.SetAnnotations(map[string]string{meta.ReconcileRequestAnnotation: tt.annotation})
			}

			ref, err := registry.ParseImageReference(imgRepo)
			g.Expect(err).ToNot(HaveOccurred())

			opts := []remote.Option{}

			tagCount, err := r.scan(context.TODO(), repo, ref, opts)
			g.Expect(err != nil).To(Equal(tt.wantErr))
			if err == nil {
				g.Expect(tagCount).To(Equal(len(tt.wantTags)))
				g.Expect(r.Database.Tags(imgRepo)).To(Equal(tt.wantTags))
				g.Expect(repo.Status.LastScanResult.TagCount).To(Equal(len(tt.wantTags)))
				g.Expect(repo.Status.LastScanResult.ScanTime).ToNot(BeZero())
				if tt.annotation != "" {
					g.Expect(repo.Status.LastHandledReconcileAt).To(Equal(tt.annotation))
				}
			}
		})
	}
}

func TestGetLatestTags(t *testing.T) {
	tests := []struct {
		name           string
		tags           []string
		wantLatestTags []string
	}{
		{
			name:           "no tags",
			wantLatestTags: nil,
		},
		{
			name:           "few semver tags",
			tags:           []string{"1.0.0", "0.0.8", "1.2.5", "3.0.1", "1.0.1"},
			wantLatestTags: []string{"3.0.1", "1.2.5", "1.0.1", "1.0.0", "0.0.8"},
		},
		{
			name:           "10 semver tags",
			tags:           []string{"1.0.0", "0.0.8", "1.2.5", "3.0.1", "1.0.1", "5.1.1", "4.1.0", "4.5.0", "4.0.3", "2.2.2"},
			wantLatestTags: []string{"5.1.1", "4.5.0", "4.1.0", "4.0.3", "3.0.1", "2.2.2", "1.2.5", "1.0.1", "1.0.0", "0.0.8"},
		},
		{
			name:           "10+ semver tags",
			tags:           []string{"1.0.0", "0.0.8", "1.2.5", "3.0.1", "1.0.1", "5.1.1", "4.1.0", "4.5.0", "4.0.3", "2.2.2", "0.5.1", "0.1.0"},
			wantLatestTags: []string{"5.1.1", "4.5.0", "4.1.0", "4.0.3", "3.0.1", "2.2.2", "1.2.5", "1.0.1", "1.0.0", "0.5.1"},
		},
		{
			name:           "few numerical tags",
			tags:           []string{"-62", "-88", "73", "72", "15"},
			wantLatestTags: []string{"73", "72", "15", "-88", "-62"},
		},
		{
			name:           "few numerical tags",
			tags:           []string{"-62", "-88", "73", "72", "15", "16", "15", "29", "-33", "-91", "100", "101"},
			wantLatestTags: []string{"73", "72", "29", "16", "15", "15", "101", "100", "-91", "-88"},
		},
		{
			name:           "few word tags",
			tags:           []string{"aaa", "bbb", "ccc"},
			wantLatestTags: []string{"ccc", "bbb", "aaa"},
		},
		{
			name:           "few word tags",
			tags:           []string{"aaa", "bbb", "ccc", "ddd", "eee", "fff", "ggg", "hhh", "iii", "jjj", "kkk", "lll"},
			wantLatestTags: []string{"lll", "kkk", "jjj", "iii", "hhh", "ggg", "fff", "eee", "ddd", "ccc"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			g.Expect(getLatestTags(tt.tags)).To(Equal(tt.wantLatestTags))
		})
	}
}

func TestFilterOutTags(t *testing.T) {
	tests := []struct {
		name     string
		tags     []string
		patterns []string
		wantErr  bool
		wantTags []string
	}{
		{
			name:     "no pattern",
			tags:     []string{"a", "b", "c", "d"},
			wantTags: []string{"a", "b", "c", "d"},
		},
		{
			name:     "single patterns",
			tags:     []string{"a", "b", "c", "d"},
			patterns: []string{"[abc]"},
			wantTags: []string{"d"},
		},
		{
			name:     "multiple patterns",
			tags:     []string{"a", "b", "c", "d"},
			patterns: []string{"[a]", "[d]"},
			wantTags: []string{"b", "c"},
		},
		{
			name:     "invalid pattern",
			patterns: []string{"[="},
			wantErr:  true,
		},
		{
			name:     "version tags",
			tags:     []string{"0.1.0", "0.2.0", "0.2.-alpha", "0.3.0", "0.4.0", "0.4.0.sig"},
			patterns: []string{"^.*\\-alpha$", "^.*\\.sig$"},
			wantTags: []string{"0.1.0", "0.2.0", "0.3.0", "0.4.0"},
		},
		{
			name:     "multiple matches in single pattern",
			tags:     []string{"aaa", "bbb", "ccc", "ddd"},
			patterns: []string{"aaa|ccc"},
			wantTags: []string{"bbb", "ddd"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			result, err := filterOutTags(tt.tags, tt.patterns)
			g.Expect(err != nil).To(Equal(tt.wantErr))
			g.Expect(result).To(Equal(tt.wantTags))
		})
	}
}

func TestIsEqualSliceContent(t *testing.T) {
	tests := []struct {
		name string
		a    []string
		b    []string
		want bool
	}{
		{
			name: "empty equal",
			want: true,
		},
		{
			name: "equal",
			a:    []string{"foo1", "bar1"},
			b:    []string{"foo1", "bar1"},
			want: true,
		},
		{
			name: "same length, different content",
			a:    []string{"foo1", "bar1"},
			b:    []string{"foo2", "bar1"},
			want: false,
		},
		{
			name: "different content length",
			a:    []string{"foo1", "bar1"},
			b:    []string{"foo1", "bar1", "baz1"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(isEqualSliceContent(tt.a, tt.b)).To(Equal(tt.want))
		})
	}
}

func TestNotify(t *testing.T) {
	nextScanMsg := "foo"
	tests := []struct {
		name       string
		beforeFunc func(oldObj, newObj *imagev1.ImageRepository)
		wantEvent  string
	}{
		{
			name: "first time success reconcile, empty old object",
			beforeFunc: func(oldObj, newObj *imagev1.ImageRepository) {
				conditions.MarkTrue(newObj, meta.ReadyCondition, meta.SucceededReason, "found x tags")
			},
			wantEvent: "Normal Succeeded found x tags",
		},
		{
			name: "no-op reconcile, same old and new object",
			beforeFunc: func(oldObj, newObj *imagev1.ImageRepository) {
				conditions.MarkTrue(oldObj, meta.ReadyCondition, meta.SucceededReason, "found x tags")
				conditions.MarkTrue(newObj, meta.ReadyCondition, meta.SucceededReason, "found x tags")
			},
			wantEvent: "Trace Succeeded foo",
		},
		{
			name: "new tags, ready but different old and new object",
			beforeFunc: func(oldObj, newObj *imagev1.ImageRepository) {
				conditions.MarkTrue(oldObj, meta.ReadyCondition, meta.SucceededReason, "found x tags")
				conditions.MarkTrue(newObj, meta.ReadyCondition, meta.SucceededReason, "found y tags")
			},
			wantEvent: "Normal Succeeded found y tags",
		},
		{
			name: "ready old object, not ready new object",
			beforeFunc: func(oldObj, newObj *imagev1.ImageRepository) {
				conditions.MarkTrue(oldObj, meta.ReadyCondition, meta.SucceededReason, "found x tags")
				conditions.MarkFalse(newObj, meta.ReadyCondition, meta.FailedReason, "scan failed")
			},
			wantEvent: "Warning Failed scan failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			recorder := record.NewFakeRecorder(32)

			oldObj := &imagev1.ImageRepository{}
			newObj := oldObj.DeepCopy()

			if tt.beforeFunc != nil {
				tt.beforeFunc(oldObj, newObj)
			}

			notify(context.TODO(), recorder, oldObj, newObj, nextScanMsg)

			select {
			case x, ok := <-recorder.Events:
				g.Expect(ok).To(Equal(tt.wantEvent != ""), "unexpected event received")
				if tt.wantEvent != "" {
					g.Expect(x).To(ContainSubstring(tt.wantEvent))
				}
			default:
				if tt.wantEvent != "" {
					t.Errorf("expected some event to be emitted")
				}
			}
		})
	}
}
