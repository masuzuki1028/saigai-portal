// Package apigen は api/openapi.yaml から生成した型とサーバーのインターフェース。
// 手で編集しない。openapi.yaml を変更したら go generate ./... を実行する。
package apigen

//go:generate go tool oapi-codegen -config ../../oapi-codegen.yaml ../../openapi.yaml
