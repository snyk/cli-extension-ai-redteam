package mocks

//go:generate mockgen -package frameworkmock -destination frameworkmock/ui_mock.go "github.com/snyk/go-application-framework/pkg/ui" UserInterface,ProgressBar
