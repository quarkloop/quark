package servicekit

import servicev1 "github.com/quarkloop/pkg/serviceapi/gen/quark/service/v1"

// CloneDescriptor returns a deep-ish copy of the descriptor. Generated proto
// Clone is intentionally avoided here to keep callers working with concrete
// types.
func CloneDescriptor(x *servicev1.ServiceDescriptor) *servicev1.ServiceDescriptor {
	if x == nil {
		return nil
	}
	out := &servicev1.ServiceDescriptor{
		Name:    x.GetName(),
		Type:    x.GetType(),
		Version: x.GetVersion(),
		Address: x.GetAddress(),
		Rpcs:    make([]*servicev1.RpcDescriptor, 0, len(x.GetRpcs())),
		Skills:  make([]*servicev1.SkillDescriptor, 0, len(x.GetSkills())),
	}
	for _, r := range x.GetRpcs() {
		out.Rpcs = append(out.Rpcs, &servicev1.RpcDescriptor{
			Service:     r.GetService(),
			Method:      r.GetMethod(),
			Request:     r.GetRequest(),
			Response:    r.GetResponse(),
			Description: r.GetDescription(),
		})
	}
	for _, s := range x.GetSkills() {
		out.Skills = append(out.Skills, &servicev1.SkillDescriptor{
			Name:     s.GetName(),
			Version:  s.GetVersion(),
			Markdown: s.GetMarkdown(),
		})
	}
	return out
}
