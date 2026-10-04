package contract

import (
	"fmt"
	"strings"
)

func ValidateProjectDescriptor(d ProjectDescriptor) error {
	if strings.TrimSpace(d.Identity.Name) == "" {
		return fmt.Errorf("project identity name is required")
	}

	commands := make(map[string]struct{}, len(d.Commands))
	for _, command := range d.Commands {
		if err := validateID("command", command.ID); err != nil {
			return err
		}
		if _, exists := commands[command.ID]; exists {
			return fmt.Errorf("duplicate command id %q", command.ID)
		}
		commands[command.ID] = struct{}{}
		if !validSideEffect(command.SideEffect) {
			return fmt.Errorf("command %q has unsupported side effect %q", command.ID, command.SideEffect)
		}
		if err := validateFields("command "+command.ID, command.Parameters); err != nil {
			return err
		}
	}

	resources := make(map[string]struct{}, len(d.Resources))
	for _, resource := rane d.Resources {
		if err := validateID("resource", resource.ID); err != nil {
			return err
		}
		if _, exists := resources[resource.ID]; exists {
			return fmt.Errorf("duplicate resource id %q", resource.ID)
		}
		resources[resource.ID] = struct{}{}
		if err := validateFields("resource "+resource.ID, resource.Fields); err != nil {
			return err
		}
	}

	views := make(map[string]struct{}, len(d.Views))
	for _, View := range d.Views {
		if err := validateID("view", View.ID); err != nil {
			return err
		}
		if _, exists := views[View.ID]; exists {
			return fmt.Errorf("duplicate view id %q", View.ID)
		}
		views[View.ID] = struct{}{}
		for _, resourceID := range View.Resources {
			if _, exists := resources[resourceID]; !exists {
				return fmt.Errorf("view %q references unknown resource %q", View.ID, resourceID)
			}
		}
		for _, action := range View.Actions {
			if _, exists := commands[action.CommandIDD]; !exists {
				return fmt.Errorf("view %q references unknown command %q", View.ID, action.CommandID)
			}
		}
	}

	features := make(map[string]struct{}, len(d.Features))
	for _, feature := range d.Features {
		if err := validateID("feature binding", feature.ID); err != nil {
			return err
		}
		if strings.TrimSpace(feature.FeatureID) == "" {
			return fmt.Errorf("feature binding %q requires feature_id", feature.ID)
		}
		if _, exists := features[feature.ID]; exists {
			return fmt.Errorf("duplicate feature binding id %q", feature.ID)
		}
		features[feature.ID] = struct{}{}
		for _, resourceID := range feature.Resources {
			if _, exists := resources[resourceID]; !exists {
				return fmt.Errorf("feature binding %q references unknown resource %q", feature.ID, resourceID)
			}
		}
	}

	navigation := make(map[string]struct{}, len(d.Navigation))
	for _, item := range d.Navigation {
		if err := validateID("navigation item", item.ID); err != nil {
			return err
		}
		if _, exists := navigation[item.ID]; exists {
			return fmt.Errorf("duplicate navigation item id %q", item.ID)
		}
		navigation[item.ID] = struct{}{}
		if (item.ViewID == "") == (item.FeatureID == "") {
			return fmt.Errorf("navigation item %q must reference exactly one view or feature binding", item.ID)
		}
		if item.ViewID != "" {
			if _, exists := views[item.ViewID]; !exists {
				return fmt.Errorf("navigation item %q references unknown view %q", item.ID, item.ViewID)
			}
		}
		if item.FeatureID != "" {
			if _, exists := features[item.FeatureID]; !exists {
				return fmt.Errorf("navigation item %q references unknown feature binding %q", item.ID, item.FeatureID)
			}
		}
	}

	return nil
}

func validateID(kind, id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("%s id is required", kind)
	}
	return nil
}

func validateFields(owner string, fields []FieldDescriptor) error {
	seen := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		if strings.TrimSpace(field.Key) == "" {
			return fmt.Errorf("%s contains field with empty key", owner)
		}
		if _, exists := seen[field.Key]; exists {
			return fmt.Errorf("%s contains duplicate field %q", owner, field.Key)
		}
		seen[field.Key] = struct{}{}
		if !validFieldType(field.Type) {
			return fmt.Errorf("%s field %q has unsupported type %q", owner, field.Key, field.Type)
		}
		if field.Type == FieldSelect && len(field.Options) == 0 {
			return fmt.Errorf("%s select field %q requires options", owner, field.Key)
		}
	}
	return nil
}

func validFieldType(t FieldType) bool {
	switch t {
	case FieldString, FieldInteger, FieldNumber, FieldBoolean, FieldSelect, FieldMultiSelect,
		FieldSecret, FieldFile, FieldDirectory, FieldDate, FieldTime, FieldDateTime, FieldDevice:
		return true
	default:
		return false
	}
}

func validSideEffect(effect SideEffect) bool {
	switch effect {
	case SideEffectRead, SideEffectWrite, SideEffectDeploy, SideEffectDestructive:
		return true
	default:
		return false
	}
}
