/*
Copyright © 2025 czx-lab www.aiweimeng.top

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package orm

import (
	"command/cmd"
	"errors"
	"maps"
	"reflect"
	"slices"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"gorm.io/gen"
	"gorm.io/gen/field"
	"gorm.io/gorm"
)

type (
	OrmConf struct {
		GenConf  gen.Config
		DataType map[string]DataTypeFn
	}
	// DataTypeFn defines a function type for custom data type mapping.
	DataTypeFn = func(gorm.ColumnType) string
	IOrmOption interface {
		apply(*OrmOption)
	}
	OrmOptionFunc func(*OrmOption)
	OrmOption     struct {
		db    *gorm.DB
		gconf gen.Config
		// Rename the file name
		rename map[string]string
		// ignore fields to be ignored
		// global ignore:
		// []string{ "*->created_at,updated_at" }
		//
		// table-specific ignore:
		// []string{ "user->created_at,updated_at" }
		// indicates ignoring the `created_at` and `updated_at` fields in the `user` table.
		ignore []string
		// rename tags
		// global retag:
		// []string{ "*->created_at->c_date" }
		//
		// table-specific retag:
		// []string{ "user->created_at->c_date" }
		// created_at@c_date indicates renaming the `created_at` JSON tag in the `user` table to `c_date`.
		retags []string
		// rename gorm tags
		// global reGromTag:
		// []string{ "*->created_at->c_date" }
		//
		// table-specific reGromTag:
		// []string{ "user->created_at->c_date" }
		// created_at@c_date indicates renaming the `created_at` Gorm tag in the `user` table to `c_date`.
		reGromTags []string
		// data type mapping
		// example:
		// global data type mapping:
		// map[string]DataTypeFn{"*->created_at": func(column gorm.ColumnType) string { return "time.Time" }}
		//
		// table-specific data type mapping:
		// map[string]DataTypeFn{"user->created_at": func(column gorm.ColumnType) string { return "time.Time" }}
		dataType map[string]DataTypeFn
		// dao generation for specified tables
		daoTables []string
		// dao generation for specified tables with API interface
		daoApi map[string]any
	}
	Orm struct {
		opt         OrmOption
		generator   *gen.Generator
		retagopt    map[string][][2]string
		regormtag   map[string][][2]string
		ignoreopt   map[string][]string
		types       map[string]map[string]DataTypeFn
		globalTypes map[string]DataTypeFn
		global      []gen.ModelOpt
		structs     []any
	}
)

func (f OrmOptionFunc) apply(o *OrmOption) {
	f(o)
}

func NewOrmCommand(opts ...IOrmOption) *Orm {
	opt := &OrmOption{}
	for _, o := range opts {
		o.apply(opt)
	}

	return &Orm{
		opt:         *opt,
		retagopt:    make(map[string][][2]string),
		ignoreopt:   make(map[string][]string),
		types:       make(map[string]map[string]DataTypeFn),
		globalTypes: make(map[string]DataTypeFn),
	}
}

// Command implements ICommand.
func (o *Orm) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "orm",
		GroupID: "db",
		Short:   "Gorm Code Generator",
		Long: `Generate Gorm model code, supporting single-table and multi-table generation.

site: https://gorm.io/gen`,
		Example: `# Generate code for a single table
command orm -t users

# Generate code for multiple tables
command orm -t users -t orders -t products

# Generate DAO code for the generated models
command orm --style dao -t users -t orders

# Generate code for all tables in the database
command orm --style model
`,
		Args: cobra.MaximumNArgs(0),
		Run:  o.run,
	}

	// Add flags
	o.flags(cmd)
	return cmd
}

// flags adds command-line flags to the Orm command.
func (o *Orm) flags(c *cobra.Command) {
	c.Flags().String("style", "model", `The file type. options: model, dao`)
	c.Flags().StringArrayP("tables", "t", nil, "List of table names to generate models for")
}

// run is the execution logic for the Orm command.
func (o *Orm) run(cmd *cobra.Command, _ []string) {
	if o.opt.db == nil {
		color.Red("\nError: Database connection is not provided\n\n")
		return
	}

	// Initialize the Gorm code generator
	o.generator = gen.NewGenerator(o.opt.gconf)
	o.generator.UseDB(o.opt.db)
	o.generator.WithJSONTagNameStrategy(func(columnName string) string {
		return columnName + ",omitempty"
	})

	// Format retag options
	if err := o.formatGlobal(); err != nil {
		color.Red("\nError formatting retags: %v\n\n", err)
		return
	}

	// Execute the code generation
	if err := o.exec(cmd.Flags()); err != nil {
		color.Red("\nError generating Gorm code: %v\n\n", err)
		return
	}
	color.Green("\nGorm code generation completed successfully.\n\n")
}

// exec executes the Orm command based on the provided flags.
func (o *Orm) exec(args *pflag.FlagSet) error {
	style, err := args.GetString("style")
	if err != nil {
		return err
	}

	tables, err := args.GetStringArray("tables")
	if err != nil {
		return err
	}
	if err := o.model(tables...); err != nil {
		return err
	}
	if style == "model" {
		goto Exec
	}

	if err := o.dao(); err != nil {
		return err
	}

Exec:
	o.generator.Execute()
	return nil
}

// dao generates DAO code for the generated models.
func (o *Orm) dao() error {
	if len(o.structs) == 0 {
		return errors.New("no structs available for DAO generation")
	}
	var structs []any
	structs_m := make(map[string]any)
	for _, meta := range o.structs {
		table := reflect.ValueOf(meta).Elem().FieldByName("TableName").String()
		if !slices.Contains(o.opt.daoTables, "*") && !slices.Contains(o.opt.daoTables, table) {
			continue
		}
		structs = append(structs, meta)
		structs_m[table] = meta
	}

	if len(structs) == 0 {
		return errors.New("no matching structs found for DAO generation")
	}

	o.generator.ApplyBasic(structs...)
	globalAnnotae, ok := o.opt.daoApi["*"]
	if ok {
		o.generator.ApplyInterface(globalAnnotae, structs...)
	}
	for table, annotae := range o.opt.daoApi {
		if table == "*" {
			continue
		}
		s, ok := structs_m[table]
		if !ok {
			continue
		}
		o.generator.ApplyInterface(annotae, s)
	}
	return nil
}

// model generates Gorm models for the specified tables.
func (o *Orm) model(tables ...string) error {
	var err error
	if len(tables) == 0 {
		tables, err = o.opt.db.Migrator().GetTables()
	}
	if err != nil {
		return err
	}

	// Prepare model generation options
	var opts []gen.ModelOpt
	if len(o.opt.ignore) > 0 {
		opts = append(opts, gen.FieldIgnore(o.opt.ignore...))
	}

	opts = append(opts, o.global...)
	for _, val := range tables {
		vals := strings.Split(val, "@")
		if len(vals) > 2 {
			color.Yellow("Skipping invalid table format: %s. Expected format: table@modelName\n", val)
			continue
		}

		// Get table-specific options
		tableopt, err := o.optByTable(vals[0])
		if err != nil {
			return err
		}
		opts = append(opts, tableopt...)

		// Apply data type mapping for the table
		o.genoptByTable(vals[0])

		if len(vals) == 1 {
			model := o.generator.GenerateModel(vals[0], opts...)
			o.structs = append(o.structs, model)
			continue
		}

		// Generate model with custom name
		model := o.generator.GenerateModelAs(vals[1], vals[0], opts...)
		o.structs = append(o.structs, model)
	}

	return nil
}

// formatGlobal processes global retag options.
func (o *Orm) formatGlobal() error {
	// Process retag options
	retags := make(map[string][][2]string)
	for _, retag := range o.opt.retags {
		parts := strings.Split(retag, "->")
		if len(parts) != 3 {
			return errors.New("invalid retag format: " + retag)
		}
		// Global retag
		if parts[0] == "*" {
			o.global = append(o.global, gen.FieldJSONTag(parts[1], parts[2]+",omitempty"))
			continue
		}
		retags[parts[0]] = append(retags[parts[0]], [2]string{parts[1], parts[2] + ",omitempty"})
	}
	o.retagopt = retags

	// Process reGromTag options
	regormtags := make(map[string][][2]string)
	for _, retag := range o.opt.reGromTags {
		parts := strings.Split(retag, "->")
		if len(parts) != 3 {
			return errors.New("invalid retag format: " + retag)
		}
		// Global reGromTag
		if parts[0] == "*" {
			o.global = append(o.global, gen.FieldGORMTag(parts[1], func(tag field.GormTag) field.GormTag {
				return tag.Set("column", parts[2])
			}))
			continue
		}
		regormtags[parts[0]] = append(retags[parts[0]], [2]string{parts[1], parts[2]})
	}

	// Process ignore options
	for _, ignore := range o.opt.ignore {
		parts := strings.Split(ignore, "->")
		if len(parts) != 2 {
			return errors.New("invalid ignore format: " + ignore)
		}
		fields := strings.Split(parts[1], ",")
		if parts[0] == "*" {
			o.global = append(o.global, gen.FieldIgnore(fields...))
			continue
		}
		o.ignoreopt[parts[0]] = append(o.ignoreopt[parts[0]], fields...)
	}

	// Process rename options
	if len(o.opt.rename) > 0 {
		o.generator.WithFileNameStrategy(func(tableName string) (fileName string) {
			if _, ok := o.opt.rename[tableName]; !ok {
				return strings.ToLower(tableName)
			}
			return o.opt.rename[tableName]
		})
	}

	// Process data type mapping options
	if o.opt.dataType == nil {
		return nil
	}
	for key, typ := range o.opt.dataType {
		parts := strings.Split(key, "->")
		if len(parts) != 2 {
			return errors.New("invalid data type mapping format: " + key)
		}
		// Global data type mapping
		if parts[0] == "*" {
			o.globalTypes[parts[1]] = typ
			continue
		}

		// Table-specific data type mapping
		if _, ok := o.types[parts[0]]; !ok {
			o.types[parts[0]] = make(map[string]DataTypeFn)
		}
		o.types[parts[0]][parts[1]] = typ
	}

	return nil
}

// optByTable retrieves retag options for a specific table.
func (o *Orm) optByTable(table string) ([]gen.ModelOpt, error) {
	var opts []gen.ModelOpt
	ign, ok := o.ignoreopt[table]
	if ok {
		opts = append(opts, gen.FieldIgnore(ign...))
	}

	if rt, ok := o.retagopt[table]; ok {
		for _, r := range rt {
			opts = append(opts, gen.FieldJSONTag(r[0], r[1]))
		}
	}

	if rgt, ok := o.regormtag[table]; ok {
		for _, r := range rgt {
			opts = append(opts, gen.FieldGORMTag(r[0], func(tag field.GormTag) field.GormTag {
				return tag.Set("column", r[1])
			}))
		}
	}

	// Data type mapping
	types_t := maps.Clone(o.globalTypes)
	if types, ok := o.types[table]; ok {
		maps.Copy(types_t, types)
	}

	if len(types_t) == 0 {
		return opts, nil
	}
	o.generator.WithDataTypeMap(types_t)
	return opts, nil
}

// genoptByTable applies data type mapping options for a specific table.
func (o *Orm) genoptByTable(table string) {
	// Data type mapping
	types, ok := o.types[table]
	if !ok {
		return
	}

	types_t := maps.Clone(o.globalTypes)
	maps.Copy(types_t, types)

	if len(types_t) == 0 {
		return
	}
	o.generator.WithDataTypeMap(types_t)
}

var _ cmd.ICommand = (*Orm)(nil)

// WithDB sets the gorm.DB instance for the Orm.
func WithDB(db *gorm.DB) IOrmOption {
	return OrmOptionFunc(func(o *OrmOption) {
		o.db = db
	})
}

// WithConfig sets the gen.Config for the Orm.
func WithConfig(gconf gen.Config) IOrmOption {
	return OrmOptionFunc(func(o *OrmOption) {
		o.gconf = gconf
	})
}

// WithRename sets the rename mapping for the Orm.
func WithRename(rename map[string]string) IOrmOption {
	return OrmOptionFunc(func(o *OrmOption) {
		o.rename = rename
	})
}

// WithIgnore sets the ignore fields for the Orm.
func WithIgnore(ignore []string) IOrmOption {
	return OrmOptionFunc(func(o *OrmOption) {
		o.ignore = ignore
	})
}

// WithRetags sets the retag options for the Orm.
func WithRetags(retags []string) IOrmOption {
	return OrmOptionFunc(func(o *OrmOption) {
		o.retags = retags
	})
}

// WithReGromTags sets the reGromTag options for the Orm.
func WithReGromTags(reGromTags []string) IOrmOption {
	return OrmOptionFunc(func(o *OrmOption) {
		o.reGromTags = reGromTags
	})
}

// WithDataType sets the data type mapping for the Orm.
func WithDataType(dataType map[string]DataTypeFn) IOrmOption {
	return OrmOptionFunc(func(o *OrmOption) {
		o.dataType = dataType
	})
}

// WithDaoTables sets the dao tables for the Orm.
func WithDaoTables(tables []string) IOrmOption {
	return OrmOptionFunc(func(o *OrmOption) {
		o.daoTables = tables
	})
}

// WithDaoApi sets the dao API interface mapping for the Orm.
func WithDaoApi(daoApi map[string]any) IOrmOption {
	return OrmOptionFunc(func(o *OrmOption) {
		o.daoApi = daoApi
	})
}
