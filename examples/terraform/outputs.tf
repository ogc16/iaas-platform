output "dashboard_url" {
  description = "Platform dashboard"
  value       = "http://${var.public_host}:${var.server_port}/"
}

output "api_docs_url" {
  description = "Swagger UI explorer"
  value       = "http://${var.public_host}:${var.server_port}/docs"
}

output "readyz_url" {
  description = "Readiness probe"
  value       = "http://${var.public_host}:${var.server_port}/readyz"
}

output "database_connection" {
  description = "Docker-internal PostgreSQL DSN"
  value       = "postgres://${var.database_user}:${var.database_password}@postgres:5432/${var.database_name}?sslmode=disable"
  sensitive   = true
}
