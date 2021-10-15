package rbac

import input.attributes.request.http as http_request

# By default, deny requests.
default allow = false

# Allow everyone to visit public urls.
allow {
    url_is_public
}

# Allow all valid users to visit exempted urls.
allow {
    url_is_exempted
    claims.name != ""
}

# Allow admins to do anything.
allow {
    user_is_admin
}

# Allow project admins to do anything under the given project.
allow {
    user_is_project_admin
}

# Allow the action if the user is granted permission to perform the action.
allow {
    some grant
    user_is_granted[grant]

    grant.method == http_request.method
    glob.match(trim(grant.endpoint, "/"), ["/"], concat("/", input.parsed_path))
}

user_is_admin {
    some role
    allewed_roles[role]

    role.name == "admin"
    role.namespace == ""
}

user_is_project_admin {
    some role
    allewed_roles[role]

    role.name == "admin"
    role.namespace == projcet_name
}

url_is_public {
    data.exemptions.public[_].method == http_request.method
    glob.match(trim(data.exemptions.public[_].endpoint, "/"), ["/"], concat("/", input.parsed_path))
}

url_is_exempted {
    data.exemptions.global[_].method == http_request.method
    glob.match(trim(data.exemptions.global[_].endpoint, "/"), ["/"], concat("/", input.parsed_path))
}

url_is_exempted {
    data.exemptions.namespaced[_].method == http_request.method
    glob.match(trim(data.exemptions.namespaced[_].endpoint, "/"), ["/"], concat("/", input.parsed_path))
    user_projects[_] == projcet_name
}

projcet_name := pn {
    pn := input.parsed_query.projectName[0]
}

roles[role] {
    some i
    data.bindings.role_bindings[i].user == claims.name
    role_refs := data.bindings.role_bindings[i].role_refs
    role := role_refs[_]
}

# only global roles and roles under the given project are allowed.
allewed_roles[role_ref] {
    role_ref := roles[_]
    role_ref.namespace == ""
}

allewed_roles[role_ref] {
    role_ref := roles[_]
    role_ref.namespace == projcet_name
}

user_is_granted[grant] {
    some role_ref
    allewed_roles[role_ref]

    some i
    data.roles.roles[i].name == role_ref.name
    data.roles.roles[i].namespace == role_ref.namespace
    grant := data.roles.roles[i].rules[_]
}

# get all projects which are visible by current user
user_projects[project] {
    project := roles[_].namespace
    project != ""
}

claims := payload {
	# TODO: Verify the signature on the Bearer token. The certificate can be
	# hardcoded into the policy, and it could also be loaded via data or
	# an environment variable. Environment variables can be accessed using
	# the `opa.runtime()` built-in function.
	# io.jwt.verify_rs256(bearer_token, certificate)

	# This statement invokes the built-in function `io.jwt.decode` passing the
	# parsed bearer_token as a parameter. The `io.jwt.decode` function returns an
	# array:
	#
	#	[header, payload, signature]
	#
	# In Rego, you can pattern match values using the `=` and `:=` operators. This
	# example pattern matches on the result to obtain the JWT payload.
	[_, payload, _] := io.jwt.decode(bearer_token)
}

bearer_token := t {
	# Bearer tokens are contained inside of the HTTP Authorization header. This rule
	# parses the header and extracts the Bearer token value. If no Bearer token is
	# provided, the `bearer_token` value is undefined.
	v := http_request.headers.authorization
	startswith(v, "Bearer ")
	t := substring(v, count("Bearer "), -1)
}
