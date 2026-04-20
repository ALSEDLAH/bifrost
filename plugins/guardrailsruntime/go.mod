module github.com/maximhq/bifrost/plugins/guardrailsruntime

go 1.26.1

require (
	github.com/maximhq/bifrost/core v1.5.2
	github.com/maximhq/bifrost/framework v1.3.2
	github.com/maximhq/bifrost/plugins/audit v0.0.0
	gorm.io/gorm v1.31.1
)

// plugins/audit isn't published as a module — consume it via the
// local source tree. Mirrors the Dockerfile.local go-workspace wiring.
replace github.com/maximhq/bifrost/plugins/audit => ../audit
