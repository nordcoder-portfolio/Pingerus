group "default" {
  targets = ["migrator", "kafka-init", "api-gateway", "scheduler", "ping-worker", "email-notifier", "frontend"]
}

target "base" {
  context = "."
}

target "migrator" {
  inherits   = ["base"]
  dockerfile = "cmd/migrator/Dockerfile"
}

target "kafka-init" {
  inherits   = ["base"]
  dockerfile = "cmd/kafka-init/Dockerfile"}

target "api-gateway" {
  inherits   = ["base"]
  dockerfile = "cmd/api-gateway/Dockerfile"
}

target "scheduler" {
  inherits   = ["base"]
  dockerfile = "cmd/scheduler/Dockerfile"
}

target "ping-worker" {
  inherits   = ["base"]
  dockerfile = "cmd/ping-worker/Dockerfile"
}

target "email-notifier" {
  inherits   = ["base"]
  dockerfile = "cmd/email-notifier/Dockerfile"
}

target "frontend" {
  context    = "frontend"
  dockerfile = "Dockerfile"
}