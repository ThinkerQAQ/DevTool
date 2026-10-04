package contract

import "testing"

func TestValidateProjectDescriptor(t *testing.T) {
	d := ProjectDescriptor{
		Identity: ProjectIdentity{Name: "example"},
		Commands: []CommandDescriptor{{
			ID: "build", Title: "Build", SideEffect: SideEffectWrite,
		}},
		Resources: []ResourceDescriptor{{ID: "environment", Title: "Environment"}},
		Views: []ViewDescriptor{{
			ID: "overview", Title: "Overview", Resources: []string{"environment"},
			Actions: []ActionDescriptor{{CommandID: "build"}},
		}},
		Navigation: []NavigationItem{{ID: "overview", Title: "Overview", ViewID: "overview"}},
	}
	if err := ValidateProjectDescriptor(d); err != nil {
		t.Fatalf("ValidateProjectDescriptor() error = %v", err)
	}
}

func TestValidateProjectDescriptorRejectsDanglingReference(t *testing.T) {
	d := ProjectDescriptor{
		Identity: ProjectIdentity{Name: "example"},
		Views:    []ViewDescriptor{{ID: "overview", Resources: []string{"missing"}}},
	}
	if err := ValidateProjectDescriptor(d); err == nil {
		t.Fatal("ValidateProjectDescriptor() expected dangling resource error")
	}
}
