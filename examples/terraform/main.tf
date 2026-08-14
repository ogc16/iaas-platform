# Deploys the IaaS Platform (PostgreSQL + server) as local containers with the
# Docker provider — a self-contained development or single-host deployment.
#
#   terraform init
#   terraform apply -var server_image_tag=v0.1.0
#
# The published image lives at ghcr.io/ogc16/iaas-platform. For a remote host,
# set the DOCKER_HOST env var (e.g. via ssh://user@host) and change the
# database volume path in variables.tf.

terraform {
  required_version = ">= 1.5"
  required_providers {
    docker = {
      source  = "kreuzwerker/docker"
      version = "~> 3.0"
    }
  }
}

provider "docker" {}

resource "docker_volume" "postgres_data" {
  name = var.database_volume
}

resource "docker_image" "iaas_platform" {
  name         = "${var.server_image}:${var.server_image_tag}"
  keep_locally = true
}

resource "docker_image" "postgres" {
  name         = "postgres:${var.postgres_version}"
  keep_locally = true
}

resource "docker_network" "iaas" {
  name = "iaas-platform-net"
}

resource "docker_container" "postgres" {
  name  = "iaas-platform-postgres"
  image = docker_image.postgres.image_id
  env = [
    "POSTGRES_USER=${var.database_user}",
    "POSTGRES_PASSWORD=${var.database_password}",
    "POSTGRES_DB=${var.database_name}",
  ]
  ports {
    internal = 5432
    external = var.database_port
  }
  volumes {
    volume_name    = docker_volume.postgres_data.name
    container_path = "/var/lib/postgresql/data"
  }
  networks_advanced {
    name = docker_network.iaas.name
  }
  healthcheck {
    test         = ["CMD-SHELL", "pg_isready -U ${var.database_user} -d ${var.database_name}"]
    interval     = "5s"
    timeout      = "3s"
    retries      = 10
  }
}

resource "docker_container" "server" {
  name  = "iaas-platform-server"
  image = docker_image.iaas_platform.image_id
  env = [
    "HTTP_ADDR=:${var.server_port}",
    "DATABASE_URL=postgres://${var.database_user}:${var.database_password}@postgres:5432/${var.database_name}?sslmode=disable",
    "JWT_SECRET=${var.jwt_secret}",
    "API_KEY_SECRET=${var.api_key_secret}",
    "PASSWORD_RESET_BASE_URL=http://${var.public_host}:${var.server_port}",
    "ENABLE_SMTP=false",
  ]
  ports {
    internal = var.server_port
    external = var.server_port
  }
  networks_advanced {
    name = docker_network.iaas.name
  }
  depends_on = [docker_container.postgres]
}
