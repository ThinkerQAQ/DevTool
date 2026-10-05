package contract

import sdkcontract "github.com/thinkerqaq/devtool/sdk/contract"

type SideEffect = sdkcontract.SideEffect

const (
	SideEffectRead        = sdkcontract.SideEffectRead
	SideEffectWrite       = sdkcontract.SideEffectWrite
	SideEffectDeploy      = sdkcontract.SideEffectDeploy
	SideEffectDestructive = sdkcontract.SideEffectDestructive
)

type FieldType = sdkcontract.FieldType

const (
	FieldString      = sdkcontract.FieldString
	FieldInteger     = sdkcontract.FieldInteger
	FieldNumber      = sdkcontract.FieldNumber
	FieldBoolean     = sdkcontract.FieldBoolean
	FieldSelect      = sdkcontract.FieldSelect
	FieldMultiSelect = sdkcontract.FieldMultiSelect
	FieldSecret      = sdkcontract.FieldSecret
	FieldFile        = sdkcontract.FieldFile
	FieldDirectory   = sdkcontract.FieldDirectory
	FieldDate        = sdkcontract.FieldDate
	FieldTime        = sdkcontract.FieldTime
	FieldDateTime    = sdkcontract.FieldDateTime
	FieldDevice      = sdkcontract.FieldDevice
)

type FieldDescriptor = sdkcontract.FieldDescriptor
type CommandDescriptor = sdkcontract.CommandDescriptor
type ResourceDescriptor = sdkcontract.ResourceDescriptor
type ActionDescriptor = sdkcontract.ActionDescriptor
type ViewDescriptor = sdkcontract.ViewDescriptor
type FeatureBinding = sdkcontract.FeatureBinding
type NavigationItem = sdkcontract.NavigationItem
type ProjectIdentity = sdkcontract.ProjectIdentity
type ProjectDescriptor = sdkcontract.ProjectDescriptor
