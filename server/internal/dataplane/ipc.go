// Package dataplane — NATS subject conventions for control-plane ↔
// data-plane IPC.
//
// Same subject layout as the Java DataPlaneIpc:
//
//   quark.control.<runtimeId>.deploy      — control → data: deploy a system
//   quark.control.<runtimeId>.undeploy    — control → data: undeploy a system
//   quark.data.status.<runtimeId>         — data → control: deploy/undeploy result
//   quark.data.heartbeat.<runtimeId>      — data → control: periodic metrics
//   quark.data.event.<runtimeId>          — data → control: lifecycle event forward
//
// Wildcards for control-plane subscription:
//
//   quark.data.event.>      — receive events from ALL data planes
//   quark.data.heartbeat.>   — receive heartbeats from ALL data planes
package dataplane

import "fmt"

// SharedRuntimeID is the runtimeId for the shared data-plane process
// (hosts all non-isolated namespaces).
const SharedRuntimeID = "shared"

// ControlPrefix is the NATS subject prefix for control → data commands.
const ControlPrefix = "quark.control."

// DataPrefix is the NATS subject prefix for data → control responses/events/heartbeats.
const DataPrefix = "quark.data."

// EventWildcard is the NATS wildcard subscription for events from ALL data planes.
const EventWildcard = "quark.data.event.>"

// HeartbeatWildcard is the NATS wildcard subscription for heartbeats from ALL data planes.
const HeartbeatWildcard = "quark.data.heartbeat.>"

// RuntimeID computes the runtimeId for a namespace.
//
// For shared namespaces: "shared" — all non-isolated namespaces run
// in the same data-plane process.
//
// For isolated namespaces: "ns-<namespace>" — a dedicated data-plane
// process per namespace.
func RuntimeID(namespace string, isIsolated bool) string {
	if isIsolated {
		return "ns-" + namespace
	}
	return SharedRuntimeID
}

// DeploySubject builds the NATS subject for a deploy command.
//
// Payload: JSON {"namespace":"alice","systemName":"monitor","source":"..."}
func DeploySubject(runtimeId string) string {
	return ControlPrefix + runtimeId + ".deploy"
}

// UndeploySubject builds the NATS subject for an undeploy command.
//
// Payload: JSON {"namespace":"alice","systemName":"monitor"}
func UndeploySubject(runtimeId string) string {
	return ControlPrefix + runtimeId + ".undeploy"
}

// StatusSubject builds the NATS subject for a data-plane status response.
func StatusSubject(runtimeId string) string {
	return DataPrefix + "status." + runtimeId
}

// HeartbeatSubject builds the NATS subject for a data-plane heartbeat.
func HeartbeatSubject(runtimeId string) string {
	return DataPrefix + "heartbeat." + runtimeId
}

// EventSubject builds the NATS subject for a forwarded lifecycle event.
func EventSubject(runtimeId string) string {
	return DataPrefix + "event." + runtimeId
}

// itoa just wraps fmt.Sprint to avoid pulling strconv into this leaf file.
// (Already imported fmt for the subject builders above.)
var _ = fmt.Sprintf
