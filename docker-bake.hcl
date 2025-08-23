group "default" {
  targets = ["migrator", "api-gateway", "scheduler", "ping-worker", "email-notifier"]
}

variable "registry" { default = "ghcr.io" }
variable "owner"    { default = "" }
variable "repo"     { default = "" }
variable "tag"      { default = "main" }
variable "context"  { default = "." }

target "base" {
  context = "${context}"
}

target "migrator" {
  inherits   = ["base"]
  dockerfile = "cmd/migrator/Dockerfile"
  tags       = ["${registry}/${owner}/${repo}/migrator:${tag}"]
}

target "api-gateway" {
  inherits   = ["base"]
  dockerfile = "cmd/api-gateway/Dockerfile"
  tags       = ["${registry}/${owner}/${repo}/api-gateway:${tag}"]
}

target "scheduler" {
  inherits   = ["base"]
  dockerfile = "cmd/scheduler/Dockerfile"
  tags       = ["${registry}/${owner}/${repo}/scheduler:${tag}"]
}

target "ping-worker" {
  inherits   = ["base"]
  dockerfile = "cmd/ping-worker/Dockerfile"
  tags       = ["${registry}/${owner}/${repo}/ping-worker:${tag}"]
}

target "email-notifier" {
  inherits   = ["base"]
  dockerfile = "cmd/email-notifier/Dockerfile"
  tags       = ["${registry}/${owner}/${repo}/email-notifier:${tag}"]
}