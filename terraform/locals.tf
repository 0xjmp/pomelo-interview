locals {
  env_vars = { for line in compact(split("\n", file(var.env_file))) :
    split("=", line)[0] => split("=", line)[1] if length(split("=", line)) == 2
  }

  # Default values if not found in .env
  google_client_id     = try(local.env_vars["GOOGLE_CLIENT_ID"], var.google_client_id)
  google_client_secret = try(local.env_vars["GOOGLE_CLIENT_SECRET"], var.google_client_secret)
  postgres_user        = try(local.env_vars["POSTGRES_USER"], var.db_user)
  postgres_db          = try(local.env_vars["POSTGRES_DB"], var.db_name)
} 