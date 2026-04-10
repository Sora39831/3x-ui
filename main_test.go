package main

import "testing"

func TestSettingCommandNeedsDBInit(t *testing.T) {
	tests := []struct {
		name string
		opts settingCommandOptions
		want bool
	}{
		{
			name: "no flags",
			opts: settingCommandOptions{},
			want: false,
		},
		{
			name: "read only show",
			opts: settingCommandOptions{show: true},
			want: true,
		},
		{
			name: "port update",
			opts: settingCommandOptions{port: 2053},
			want: true,
		},
		{
			name: "db type only",
			opts: settingCommandOptions{dbType: "mariadb"},
			want: false,
		},
		{
			name: "db host only",
			opts: settingCommandOptions{dbHost: "127.0.0.1"},
			want: false,
		},
		{
			name: "node role only",
			opts: settingCommandOptions{nodeRoleSet: true},
			want: false,
		},
		{
			name: "show with db config update",
			opts: settingCommandOptions{show: true, dbType: "mariadb"},
			want: true,
		},
		{
			name: "telegram update",
			opts: settingCommandOptions{tgbotToken: "token"},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.opts.needsDBInit(); got != tt.want {
				t.Fatalf("needsDBInit() = %v, want %v", got, tt.want)
			}
		})
	}
}
