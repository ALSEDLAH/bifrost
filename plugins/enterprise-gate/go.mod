module github.com/maximhq/bifrost/plugins/enterprise-gate

go 1.26.1

require (
	github.com/maximhq/bifrost/core v1.5.2
	github.com/maximhq/bifrost/framework v1.3.2
	github.com/maximhq/bifrost/plugins/audit v0.0.0-00010101000000-000000000000
	github.com/maximhq/bifrost/plugins/license v0.0.0-00010101000000-000000000000
	gorm.io/gorm v1.31.1
)

// During development the audit + license plugins are sibling modules
// with no published version yet. Operators should add a workspace replace
// in their go.work; CI uses go.mod replace directives:
replace github.com/maximhq/bifrost/plugins/audit => ../audit
replace github.com/maximhq/bifrost/plugins/license => ../license
