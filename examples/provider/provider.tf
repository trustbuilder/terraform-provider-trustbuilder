provider "trustbuilder" {
  uri = "https://tenant-manager"
  jwt_hashed_token = {
    claims_json = jsonencode({
      sub = "subject"
      scope = [
        "scope:read",
      ]
      iss = "issuer"
    })
    secret                     = local.jwt_secret
    algorithm                  = "HS256"
    validation_duration_minute = 10
  }
}
