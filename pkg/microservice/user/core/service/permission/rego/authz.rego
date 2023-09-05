package rbac

import input.attributes.request.http as http_request

# Policy rule definitions in rbac style, which is consumed by OPA server.
# you can use it to:
# 1. decide if a request is allowed and get status code and additional headers(if any) by querying: rbac.response
# 2. get all visible projects for an authenticated user by querying: rbac.user_visible_projects
# 3. get all allowed projects for a certain action(method+endpoint) for an authenticated user by querying: rbac.user_allowed_projects
# 4. check if a user is system admin by querying: rbac.user_is_admin
# 5. check if a user is project admin by querying: rbac.user_is_project_admin

default response = {
  "allowed": true
}

response = r {
    not url_is_public
    not is_authenticated
    r := {
      "allowed": false,
      "http_status": 401
    }
}

# public urls are visible for all users
url_is_public {
    some i
    data.exemptions[i].method == http_request.method
    glob.match(trim(data.exemptions.public[i].endpoint, "/"), ["/"], concat("/", input.parsed_path))
}

claims := payload {
	# Verify the signature on the Bearer token. The certificate can be
	# hardcoded into the policy, and it could also be loaded via data or
	# an environment variable. Environment variables can be accessed using
	# the `opa.runtime()` built-in function.
	io.jwt.verify_hs256(bearer_token, secret)

	# This statement invokes the built-in function `io.jwt.decode` passing the
	# parsed bearer_token as a parameter. The `io.jwt.decode` function returns an
	# array:
	#
	#	[header, payload, signature]
	#
	# In Rego, you can pattern match values using the `=` and `:=` operators. This
	# example pattern matches on the result to obtain the JWT payload.
	[_, payload, _] := io.jwt.decode(bearer_token)

    # it is not working, don't know why
	# [valid, _, payload] := io.jwt.decode_verify(bearer_token, {
    #     "secret": secret,
    #     "alg": "alg",
    # })
}

bearer_token_in_header := t {
	# Bearer tokens are contained inside of the HTTP Authorization header. This rule
	# parses the header and extracts the Bearer token value. If no Bearer token is
	# provided, the `bearer_token` value is undefined.
	v := http_request.headers.authorization
	startswith(v, "Bearer ")
	t := substring(v, count("Bearer "), -1)
}

bearer_token = t {
    t := bearer_token_in_header
}

bearer_token = t {
    not bearer_token_in_header
    t := input.parsed_query.token[0]
}

is_authenticated {
    claims
    claims.uid != ""
    claims.exp > time.now_ns()/1000000000
}

envs := env {
    env := opa.runtime()["env"]
}

secret := s {
    s := envs["SECRET_KEY"]
}
