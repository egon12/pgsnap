package docker

var migrationPath = "./sqlschema/"

// SetMigrationPath set migration path
func SetMigrationPath(path string) {
	migrationPath = path
}
