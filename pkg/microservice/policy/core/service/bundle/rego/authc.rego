package authc


import input.attributes.request.http as http_request

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
