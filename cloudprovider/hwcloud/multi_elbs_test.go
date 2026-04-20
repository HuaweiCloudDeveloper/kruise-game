package hwcloud

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

func TestMultiElbsPlugin_DrainPendingDealloc_ReleasesPortsWhenServicesGone(t *testing.T) {
	m := &MultiElbsPlugin{
		minPort:        6000,
		maxPort:        6001,
		cache:          [][]bool{make([]bool, 2)},
		podAllocate:    make(map[string]*lbsPorts),
		pendingDealloc: make(map[string]*pendingDeallocEntry),
		mutex:          sync.RWMutex{},
	}
	m.cache[0][0] = true // port 6000 is occupied

	conf := &multiELBsConfig{
		lbNames: map[string]string{
			"lb-1": "ELB-1",
		},
	}
	m.podAllocate["default/pod-a"] = &lbsPorts{
		index: 0,
		lbIds: []string{"lb-1"},
		ports: []int32{6000},
	}

	m.markPendingDealloc("default/pod-a", "default", "pod-a", conf)
	entry := m.pendingDealloc["default/pod-a"]
	assert.NotNil(t, entry)
	assert.Equal(t, []types.NamespacedName{{Namespace: "default", Name: "pod-a-elb-1"}}, entry.svcKeys)

	clientMock := new(MockClient)
	clientMock.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(k8serrors.NewNotFound(schema.GroupResource{}, "pod-a-elb-1"))

	m.tryDrainPendingDealloc(clientMock, context.Background(), 1)

	assert.False(t, m.cache[0][0])
	_, stillPending := m.pendingDealloc["default/pod-a"]
	assert.False(t, stillPending)
}

func TestMultiElbsPlugin_DrainPendingDealloc_DoesNotReleaseWhenServiceStillExists(t *testing.T) {
	m := &MultiElbsPlugin{
		minPort:        6000,
		maxPort:        6001,
		cache:          [][]bool{make([]bool, 2)},
		podAllocate:    make(map[string]*lbsPorts),
		pendingDealloc: make(map[string]*pendingDeallocEntry),
		mutex:          sync.RWMutex{},
	}
	m.cache[0][0] = true

	conf := &multiELBsConfig{
		lbNames: map[string]string{
			"lb-1": "ELB-1",
		},
	}
	m.podAllocate["default/pod-a"] = &lbsPorts{
		index: 0,
		lbIds: []string{"lb-1"},
		ports: []int32{6000},
	}
	m.markPendingDealloc("default/pod-a", "default", "pod-a", conf)

	clientMock := new(MockClient)
	clientMock.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			svc := args[2].(*corev1.Service)
			svc.Namespace = "default"
			svc.Name = "pod-a-elb-1"
		}).
		Return(nil)

	m.tryDrainPendingDealloc(clientMock, context.Background(), 1)

	assert.True(t, m.cache[0][0])
	_, stillPending := m.pendingDealloc["default/pod-a"]
	assert.True(t, stillPending)
}

func TestMultiElbsPlugin_Allocate_ReusesPendingOnSameNameRecreate(t *testing.T) {
	m := &MultiElbsPlugin{
		minPort:        6000,
		maxPort:        6001,
		cache:          [][]bool{make([]bool, 2)},
		podAllocate:    make(map[string]*lbsPorts),
		pendingDealloc: make(map[string]*pendingDeallocEntry),
		mutex:          sync.RWMutex{},
	}
	lbs := &lbsPorts{
		index: 0,
		lbIds: []string{"lb-1"},
		ports: []int32{6000},
	}
	m.pendingDealloc["default/pod-a"] = &pendingDeallocEntry{
		lbs:     lbs,
		svcKeys: []types.NamespacedName{{Namespace: "default", Name: "pod-a-elb-1"}},
	}

	conf := &multiELBsConfig{}
	got, err := m.allocate(conf, "default/pod-a")
	assert.NoError(t, err)
	assert.Same(t, lbs, got)
	assert.Nil(t, m.pendingDealloc["default/pod-a"])
	assert.Same(t, lbs, m.podAllocate["default/pod-a"])
}
