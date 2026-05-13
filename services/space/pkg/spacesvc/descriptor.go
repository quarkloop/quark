package spacesvc

import (
	servicev1 "github.com/quarkloop/pkg/serviceapi/gen/quark/service/v1"
	spacev1 "github.com/quarkloop/pkg/serviceapi/gen/quark/space/v1"
)

func Descriptor(address string, skill *servicev1.SkillDescriptor) *servicev1.ServiceDescriptor {
	var skills []*servicev1.SkillDescriptor
	if skill != nil {
		skills = append(skills, skill)
	}
	return &servicev1.ServiceDescriptor{
		Name:    "space",
		Type:    "space",
		Version: "1.0.0",
		Address: address,
		Rpcs: []*servicev1.RpcDescriptor{
			{Service: spacev1.SpaceService_ServiceDesc.ServiceName, Method: "CreateSpace", Request: "quark.space.v1.CreateSpaceRequest", Response: "quark.space.v1.Space", Description: "Create a space and persist its initial Quarkfile."},
			{Service: spacev1.SpaceService_ServiceDesc.ServiceName, Method: "UpdateQuarkfile", Request: "quark.space.v1.UpdateQuarkfileRequest", Response: "quark.space.v1.Space", Description: "Replace the latest Quarkfile for a space."},
			{Service: spacev1.SpaceService_ServiceDesc.ServiceName, Method: "GetSpace", Request: "quark.space.v1.GetSpaceRequest", Response: "quark.space.v1.Space", Description: "Return persisted space metadata."},
			{Service: spacev1.SpaceService_ServiceDesc.ServiceName, Method: "ListSpaces", Request: "google.protobuf.Empty", Response: "quark.space.v1.ListSpacesResponse", Description: "List registered spaces."},
			{Service: spacev1.SpaceService_ServiceDesc.ServiceName, Method: "DeleteSpace", Request: "quark.space.v1.DeleteSpaceRequest", Response: "google.protobuf.Empty", Description: "Delete a space and its service-owned data."},
			{Service: spacev1.SpaceService_ServiceDesc.ServiceName, Method: "GetQuarkfile", Request: "quark.space.v1.GetQuarkfileRequest", Response: "quark.space.v1.QuarkfileResponse", Description: "Return the authoritative Quarkfile bytes."},
			{Service: spacev1.SpaceService_ServiceDesc.ServiceName, Method: "GetAgentEnvironment", Request: "quark.space.v1.GetAgentEnvironmentRequest", Response: "quark.space.v1.AgentEnvironmentResponse", Description: "Resolve model environment entries for runtime launch."},
			{Service: spacev1.SpaceService_ServiceDesc.ServiceName, Method: "GetSpacePaths", Request: "quark.space.v1.GetSpacePathsRequest", Response: "quark.space.v1.SpacePaths", Description: "Return derived storage paths for a space."},
			{Service: spacev1.SpaceService_ServiceDesc.ServiceName, Method: "Doctor", Request: "quark.space.v1.DoctorRequest", Response: "quark.space.v1.DoctorResponse", Description: "Run Quarkfile and installed-plugin diagnostics."},
		},
		Skills: skills,
	}
}
