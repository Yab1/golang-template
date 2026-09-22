package event

const (
	TypeUserRegistered        = "dev.yab1.golangtemplate.user.registered.v1"
	TypeUserActivated         = "dev.yab1.golangtemplate.user.activated.v1"
	TypeUserPasswordChanged   = "dev.yab1.golangtemplate.user.password-changed.v1"
	TypeUserVisibilityChanged = "dev.yab1.golangtemplate.user.visibility-changed.v1"

	TypePostCreated           = "dev.yab1.golangtemplate.post.created.v1"
	TypePostUpdated           = "dev.yab1.golangtemplate.post.updated.v1"
	TypePostVisibilityChanged = "dev.yab1.golangtemplate.post.visibility-changed.v1"
	TypePostDeleted           = "dev.yab1.golangtemplate.post.deleted.v1"

	TypeFileUploaded = "dev.yab1.golangtemplate.file.uploaded.v1"
	TypeFileDeleted  = "dev.yab1.golangtemplate.file.deleted.v1"
)

const (
	SchemaUserRegistered        = "/docs/eventing/schemas/user-registered.v1.schema.json"
	SchemaUserActivated         = "/docs/eventing/schemas/user-activated.v1.schema.json"
	SchemaUserPasswordChanged   = "/docs/eventing/schemas/user-password-changed.v1.schema.json"
	SchemaUserVisibilityChanged = "/docs/eventing/schemas/user-visibility-changed.v1.schema.json"
	SchemaPostCreated           = "/docs/eventing/schemas/post-created.v1.schema.json"
	SchemaPostUpdated           = "/docs/eventing/schemas/post-updated.v1.schema.json"
	SchemaPostVisibilityChanged = "/docs/eventing/schemas/post-visibility-changed.v1.schema.json"
	SchemaPostDeleted           = "/docs/eventing/schemas/post-deleted.v1.schema.json"
	SchemaFileUploaded          = "/docs/eventing/schemas/file-uploaded.v1.schema.json"
	SchemaFileDeleted           = "/docs/eventing/schemas/file-deleted.v1.schema.json"
)
