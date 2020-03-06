package debugproxies

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/ericchiang/k8s"
	corev1 "github.com/ericchiang/k8s/apis/core/v1"
	"gopkg.in/inconshreveable/log15.v2"
)

// Represents an endpoint
type Endpoint struct {
	// Service to which the endpoint belongs
	Service string
	// Host:port, so hostname part of a URL (ip address ok)
	Host string
}

// ScanConsumer is the callback to consume scan results.
type ScanConsumer func([]Endpoint)

// clusterScanner scans the cluster for endpoints belonging to services that have annotation sourcegraph.prometheus/scrape=true.
// It runs an event loop that reacts to changes to the endpoints set. Everytime there is a change it calls the ScanConsumer.
type clusterScanner struct {
	client  *k8s.Client
	consume ScanConsumer
}

// Starts a cluster scanner with the specified consumer. Does not block.
func StartClusterScanner(consumer ScanConsumer) error {
	client, err := k8s.NewInClusterClient()
	if err != nil {
		return err
	}

	cs := &clusterScanner{
		client:  client,
		consume: consumer,
	}

	go cs.runEventLoop()
	return nil
}

// Runs the k8s.Watch endpoints event loop, and triggers a rescan of cluster when something changes with endpoints.
// Before spinning in the loop does an initial scan.
func (cs *clusterScanner) runEventLoop() {
	cs.scanCluster()
	for {
		err := cs.watchEndpointEvents()
		log15.Debug("failed to watch kubernetes endpoints", "error", err)
		time.Sleep(time.Second * 5)
	}
}

// watchEndpointEvents uses the k8s watch API operation to watch for endpoint events. Spins forever unless an error
// occurs that would necessitate creating a new watcher. The caller will then call again creating the new watcher.
func (cs *clusterScanner) watchEndpointEvents() error {
	watcher, err := cs.client.Watch(context.Background(), cs.client.Namespace, new(corev1.Endpoints))
	if err != nil {
		return fmt.Errorf("k8s client.Watch error: %w", err)
	}
	defer watcher.Close()

	for {
		var eps corev1.Endpoints
		eventType, err := watcher.Next(&eps)
		if err != nil {
			// we need a new watcher
			return fmt.Errorf("k8s watcher.Next error: %w", err)
		}

		if eventType == k8s.EventError {
			// we need a new watcher
			return errors.New("error event")
		}

		cs.scanCluster()
	}
}

// scanCluster looks for endpoints belonging to services that have annotation sourcegraph.prometheus/scrape=true.
// It derives the appropriate port from the prometheus.io/port annotation.
func (cs *clusterScanner) scanCluster() {
	var services corev1.ServiceList

	err := cs.client.List(context.Background(), cs.client.Namespace, &services)
	if err != nil {
		log15.Error("k8s failed to list services", "error", err)
		return
	}

	var scanResults []Endpoint

	for _, svc := range services.Items {
		svcName := *svc.Metadata.Name

		// TODO(uwedeportivo): pgsql doesn't work, figure out why
		if svcName == "pgsql" {
			continue
		}

		if svc.Metadata.Annotations["sourcegraph.prometheus/scrape"] != "true" {
			continue
		}

		portStr := svc.Metadata.Annotations["prometheus.io/port"]
		if portStr == "" {
			continue
		}
		port, err := strconv.Atoi(portStr)
		if err != nil {
			log15.Debug("k8s prometheus.io/port annotation for service is not an integer", "service", svcName, "port", portStr)
			continue
		}

		var endpoints corev1.Endpoints
		err = cs.client.Get(context.Background(), cs.client.Namespace, svcName, &endpoints)
		if err != nil {
			log15.Error("k8s failed to get endpoints", "error", err)
			return
		}

		for _, subset := range endpoints.Subsets {
			for _, addr := range subset.Addresses {
				host := addrToHost(addr, port)
				if host != "" {
					scanResults = append(scanResults, Endpoint{
						Service: svcName,
						Host:    host,
					})
				}
			}
		}
	}

	cs.consume(scanResults)
}

// addrToHost converts a scanned k8s endpoint address structure into a string that is the host:port part of a URL.
func addrToHost(addr *corev1.EndpointAddress, port int) string {
	if addr.Ip != nil {
		return fmt.Sprintf("%s:%d", *addr.Ip, port)
	} else if addr.Hostname != nil && *addr.Hostname != "" {
		return fmt.Sprintf("%s:%d", *addr.Hostname, port)
	}
	return ""
}
