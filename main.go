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
package main

import (
	"command/annotae"
	"command/cmd"
	"command/cmd/encrypt"
	"command/cmd/orm"

	"gorm.io/driver/mysql"
	"gorm.io/gen"
	"gorm.io/gorm"
)

func main() {
	gormdb, err := gorm.Open(mysql.Open("root:root@tcp(127.0.0.1:3306)/amg?charset=utf8mb4&loc=Local&parseTime=True"))
	if err != nil {
		panic(err)
	}

	timeFunc := func(detailType gorm.ColumnType) (dataType string) {
		switch detailType.Name() {
		case "created_at":
			return "types.DbTime"
		}
		return "time.Time"
	}
	cmds := []cmd.ICommand{
		orm.NewOrmCommand(
			orm.WithConfig(gen.Config{
				OutPath:           "./db/dao",
				OutFile:           "",
				ModelPkgPath:      "./model",
				Mode:              gen.WithDefaultQuery | gen.WithoutContext | gen.WithQueryInterface,
				FieldNullable:     false,
				FieldCoverable:    false,
				FieldSignable:     false,
				FieldWithIndexTag: false,
				FieldWithTypeTag:  true,
			}),
			orm.WithDB(gormdb),
			orm.WithDataType(map[string]orm.DataTypeFn{
				"*->timestamp": timeFunc,
			}),
			orm.WithIgnore([]string{"*->created_at,updated_at"}),
			orm.WithRename(map[string]string{"user": "user_base"}),
			orm.WithRetags([]string{"*->created_at->c_date", "*->updated_at->u_date"}),
			orm.WithReGromTags([]string{"*->created_at->-", "*->updated_at->-"}),
			orm.WithDaoTables([]string{"user", "game"}),
			orm.WithDaoApi(map[string]any{
				"*": func(annotae.Querier) {},
			}),
		),
		encrypt.NewRSA(),
	}
	cmd.Execute(cmds...)
}
