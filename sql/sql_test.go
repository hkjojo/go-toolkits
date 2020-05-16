package sql

import (
	"testing"
)

func TestGetGormRows(t *testing.T) {
	type user struct {
		ID       string `gorm:"index"`
		UserName string `json:"-"`
		Phone    string
		Temp     string `gorm:"-"`
	}
	ret := PrepareInsertSQL("t_user", &user{}, "id")
	if ret != "INSERT INTO `t_user` (`user_name`,`phone`) VALUES(?,?)" {
		t.Fail()
	}
}
