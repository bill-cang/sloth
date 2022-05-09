/*
@Time   : 2022/5/7 16:42
@Author : ckx0709
@Remark :
*/
package model

var GetterTemplate = `
func ({{.Receiver}} *{{.Struct}}) Get{{.Field}}() {{.Type}} {
	return {{.Receiver}}.{{.Field}}
}
`
