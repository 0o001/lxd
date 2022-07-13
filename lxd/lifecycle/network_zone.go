package lifecycle

import (
	"fmt"
	"net/url"

	"github.com/lxc/lxd/lxd/project"
	"github.com/lxc/lxd/shared/api"
)

// Internal copy of the network zone interface.
type networkZone interface {
	Info() *api.NetworkZone
	Project() string
}

// NetworkZoneAction represents a lifecycle event action for network zones.
type NetworkZoneAction string

// NetworkZoneRecordAction represents a lifecycle event action for network zone records.
type NetworkZoneRecordAction string

// All supported lifecycle events for network zones.
const (
	NetworkZoneCreated = NetworkZoneAction(api.EventLifecycleNetworkZoneCreated)
	NetworkZoneDeleted = NetworkZoneAction(api.EventLifecycleNetworkZoneDeleted)
	NetworkZoneUpdated = NetworkZoneAction(api.EventLifecycleNetworkZoneUpdated)

	NetworkZoneRecordCreated = NetworkZoneRecordAction(api.EventLifecycleNetworkZoneRecordCreated)
	NetworkZoneRecordDeleted = NetworkZoneRecordAction(api.EventLifecycleNetworkZoneRecordDeleted)
	NetworkZoneRecordUpdated = NetworkZoneRecordAction(api.EventLifecycleNetworkZoneRecordUpdated)
)

// Event creates the lifecycle event for an action on a network zone.
func (a NetworkZoneAction) Event(n networkZone, requestor *api.EventLifecycleRequestor, ctx map[string]any) api.EventLifecycle {
	u := fmt.Sprintf("/1.0/network-zones/%s", url.PathEscape(n.Info().Name))
	if n.Project() != project.Default {
		u = fmt.Sprintf("%s?project=%s", u, url.QueryEscape(n.Project()))
	}

	return api.EventLifecycle{
		Action:    string(a),
		Source:    u,
		Context:   ctx,
		Requestor: requestor,
	}
}

// Event creates the lifecycle event for an action on a network zone record.
func (a NetworkZoneRecordAction) Event(n networkZone, name string, requestor *api.EventLifecycleRequestor, ctx map[string]any) api.EventLifecycle {
	u := fmt.Sprintf("/1.0/network-zones/%s/records/%s", url.PathEscape(n.Info().Name), url.PathEscape(name))
	if n.Project() != project.Default {
		u = fmt.Sprintf("%s?project=%s", u, url.QueryEscape(n.Project()))
	}

	return api.EventLifecycle{
		Action:    string(a),
		Source:    u,
		Context:   ctx,
		Requestor: requestor,
	}
}
