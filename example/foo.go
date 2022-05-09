/*
@Time   : 2022/5/7 10:53
@Author : ckx0709
@Remark :
*/
//go:generate  sloth -out=Bloc,Office -fun=set,get
package example

type Bloc struct {
	Office
	Name       string `gorm:"column:name;not null;comment:名称"`
	Logo       string `gorm:"column:logo;comment:logo"`
	Master     string `gorm:"column:master;comment:负责人"`
	Phone      string `gorm:"column:phone;comment:电话"`
	Email      string `gorm:"column:email;comment:邮箱"`
	ProvinceID string `gorm:"column:province_id;comment:所属省id"`
	CityID     string `gorm:"column:city_id;comment:所属市id"`
	Address    string `gorm:"column:address;comment:地址"`
}

type Office struct {
	Name       string `gorm:"column:name;not null;comment:名称"`
	Logo       string `gorm:"column:logo;comment:机构logo"`
	Master     string `gorm:"column:master;comment:负责人"`
	Email      string `gorm:"column:email;comment:邮箱"`
	Phone      string `gorm:"column:phone;comment:电话"`
	Address    string `gorm:"column:address;comment:联系地址"`
	ProvinceID string `gorm:"column:province_id;comment:所属省id"`
	CityID     string `gorm:"column:city_id;comment:所属市id"`
}
