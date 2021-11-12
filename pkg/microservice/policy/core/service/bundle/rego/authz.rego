package rbac

import input.attributes.request.http as http_request

# Policy rule definitions in rbac style, which is consumed by OPA server.
# you can use it to:
# 1. decide if a request is allowed by querying: rbac.allow
# 2. decide if a request is allowed and get the status code by querying: rbac.response
# 3. get all visible projects for an authenticated user by querying: rbac.user_visible_projects
# 4. get all allowed projects for a certain action(method+endpoint) for an authenticated user by querying: rbac.user_allowed_projects
# 5. check if a user is system admin by querying: rbac.user_is_admin
# 6. check if a user is project admin by querying: rbac.user_is_project_admin

default response = {
  "allowed": false,
  "http_status": 403
}

response = r {
  not is_authenticated
  not url_is_public
  r := {
    "allowed": false,
    "http_status": 401
  }
}

response = r {
  allow
  r := {
    "allowed": true,
  }
}

# By default, deny requests.
default allow = false

# Allow everyone to visit public urls.
allow {
    url_is_public
}

allow {
    is_authenticated
    access_is_granted

}

# Allow all valid users to visit exempted urls.
access_is_granted {
    url_is_exempted
}

# Allow admins to do anything.
access_is_granted {
    user_is_admin
}

# Allow project admins to do anything under the given project.
access_is_granted {
    not url_is_privileged
    user_is_project_admin
}

# Allow the action if the user is granted permission to perform the action.
access_is_granted {
    not url_is_privileged

    some grant
    user_is_granted[grant]

    grant.method == http_request.method
    glob.match(trim(grant.endpoint, "/"), ["/"], concat("/", input.parsed_path))
}

user_is_admin {
    some role
    all_roles[role]

    role.name == "admin"
    role.namespace == "*"
}

user_is_project_admin {
    some role
    allowed_roles[role]

    role.name == "admin"
    #role.namespace == project_name
}

# public urls are visible for all users
url_is_public {
    some i
    data.exemptions.public[i].method == http_request.method
    glob.match(trim(data.exemptions.public[i].endpoint, "/"), ["/"], concat("/", input.parsed_path))
}

# exempted urls are visible for all authenticated users
url_is_exempted {
    not url_is_registered
}

url_is_registered {
    some i
    data.exemptions.registered[i].method == http_request.method
    glob.match(trim(data.exemptions.registered[i].endpoint, "/"), ["/"], concat("/", input.parsed_path))
}

# privileged urls are visible for system admins only
url_is_privileged {
    some i
    data.exemptions.privileged[i].method == http_request.method
    glob.match(trim(data.exemptions.privileged[i].endpoint, "/"), ["/"], concat("/", input.parsed_path))
}

project_name := pn {
    pn := input.parsed_query.projectName[0]
}

# get all projects which are visible by current user
user_projects[project] {
    some i
    data.bindings.role_bindings[i].uid == claims.uid
    project := data.bindings.role_bindings[i].bindings[_].namespace
}

# get all projects which are visible by all users (the user name is "*")
user_projects[project] {
    some i
    data.bindings.role_bindings[i].uid == "*"
    project := data.bindings.role_bindings[i].bindings[_].namespace
}

# all projects which are allowed by current user
user_allowed_projects[project] {
    some project
    user_projects[project]
    not user_is_admin
    allow with project_name as project
}

# if user is system admin, return all projects
user_allowed_projects[project] {
    project := "*"
    user_is_admin
}

# all projects which are visible by current user
user_visible_projects[project] {
    some project
    user_projects[project]
    not user_is_admin
}

# if user is system admin, return all projects
user_visible_projects[project] {
    project := "*"
    user_is_admin
}


all_roles[role_ref] {
    some i
    data.bindings.role_bindings[i].uid == claims.uid
    role_ref := data.bindings.role_bindings[i].bindings[j].role_refs[_]
}

# only roles under the given project are allowed
allowed_roles[role_ref] {
    some i
    data.bindings.role_bindings[i].uid == claims.uid
    data.bindings.role_bindings[i].bindings[j].namespace == project_name
    role_ref := data.bindings.role_bindings[i].bindings[j].role_refs[_]
}

# if the proejct is visible by all users (the user name is "*"), the bound roles are also allowed
allowed_roles[role_ref] {
    some i
    data.bindings.role_bindings[i].uid == "*"
    data.bindings.role_bindings[i].bindings[_].namespace == project_name
    role_ref := data.bindings.role_bindings[i].bindings[j].role_refs[_]
}

user_is_granted[grant] {
    some role_ref
    allowed_roles[role_ref]

    some i
    data.roles.roles[i].name == role_ref.name
    data.roles.roles[i].namespace == role_ref.namespace
    grant := data.roles.roles[i].rules[_]
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
