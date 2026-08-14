variable "server_image" {
  description = "Container image for the IaaS Platform server binary"
  type        = string
  default     = "ghcr.io/ogc16/iaas-platform"
}

variable "server_image_tag" {
  description = "Image tag to deploy (see releases)"
  type        = string
  default     = "latest"
}

variable "postgres_version" {
  description = "PostgreSQL image tag"
  type        = string
  default     = "16-alpine"
}

variable "database_user" {
  description = "PostgreSQL superuser name"
  type        = string
  default     = "iaas"
}

variable "database_password" {
  description = "PostgreSQL password"
  type        = string
  sensitive   = true
  default     = "iaas"
}

variable "database_name" {
  description = "PostgreSQL database name"
  type        = string
  default     = "iaas"
}

variable "database_port" {
  description = "Host port exposed for PostgreSQL"
  type        = number
  default     = 5432
}

variable "database_volume" {
  description = "Docker volume name holding PostgreSQL data"
  type        = string
  default     = "iaas-platform-pgdata"
}

variable "server_port" {
  description = "Port the platform serves on"
  type        = number
  default     = 8080
}

variable "public_host" {
  description = "Hostname clients use to reach the platform"
  type        = string
  default     = "localhost"
}

variable "jwt_secret" {
  description = "Secret used to sign session JWTs (rotate regularly)"
  type        = string
  sensitive   = true
}

variable "api_key_secret" {
  description = "Secret used to derive API keys (rotate regularly)"
  type        = string
  sensitive   = true
}
