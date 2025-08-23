group "default" {
  targets = ["migrator", "api-gateway", "scheduler", "ping-worker", "email-notifier"]
}

variable "context" { default = "." }

target "base" {
  context = "${context}"
}

target "migrator" {
  inherits   = ["base"]
  dockerfile = "cmd/migrator/Dockerfile"
  tags       = ["migrator:local"]
}

target "api-gateway" {
  inherits   = ["base"]
  dockerfile = "cmd/api-gateway/Dockerfile"
  tags       = ["api-gateway:local"]
}

target "scheduler" {
  inherits   = ["base"]
  dockerfile = "cmd/scheduler/Dockerfile"
  tags       = ["scheduler:local"]
}

target "ping-worker" {
  inherits   = ["base"]
  dockerfile = "cmd/ping-worker/Dockerfile"
  tags       = ["ping-worker:local"]
}

target "email-notifier" {
  inherits   = ["base"]
  dockerfile = "cmd/email-notifier/Dockerfile"
  tags       = ["email-notifier:local"]
}
