/*
@Time   : 2022/5/7 16:40
@Author : ckx0709
@Remark :
*/
package model

var SetterTemplate = `
func ({{.Receiver}} *{{.Struct}}) Set{{.Field}}(val {{.Type}}) {
	{{.Receiver}}.{{.Field}} = val
	//{{.Receiver}}.Update("{{.Column}}",val)
}`
