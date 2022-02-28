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
    roles := all_roles
    r := {
      "allowed": true,
      "headers": {
        "Roles": json.marshal(roles),
      }
    }
}

# response for resource filtering, all allowed resources IDs will be returned in headers
response = r {
    is_authenticated
    not allow
    rule_is_matched_for_filtering
    roles := all_roles
    role_resource := user_role_allowed_resources
    policy_resource := user_policy_allowed_resources
    resource := role_resource | policy_resource
    policy_rule := user_matched_policy_rule
    role_rule := user_matched_role_rule
    rule := policy_rule | role_rule
    r := {
      "allowed": true,
      "headers": {
        "Resources": json.marshal(resource),
        "Roles": json.marshal(roles),
        "Rules": json.marshal(rule)
      }
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

    some rule

    allowed_role_plain_rules[rule]
    rule.method == http_request.method
    glob.match(trim(rule.endpoint, "/"), ["/"], concat("/", input.parsed_path))
}

access_is_granted {
    not url_is_privileged

    some rule

    allowed_policy_plain_rules[rule]
    rule.method == http_request.method
    glob.match(trim(rule.endpoint, "/"), ["/"], concat("/", input.parsed_path))
}

access_is_granted {
    some rule

    allowed_role_attributive_rules[rule]
    rule.method == http_request.method
    glob.match(trim(rule.endpoint, "/"), ["/"], concat("/", input.parsed_path))

    all_attributes_match(rule.matchAttributes, rule.resourceType, get_resource_id(rule.idRegex))
}

access_is_granted {
    some rule

    allowed_policy_attributive_rules[rule]
    rule.method == http_request.method
    glob.match(trim(rule.endpoint, "/"), ["/"], concat("/", input.parsed_path))

    all_attributes_match(rule.matchAttributes, rule.resourceType, get_resource_id(rule.idRegex))
}

rule_is_matched_for_filtering {
    count(user_matched_role_rule_for_filtering) > 0
}

rule_is_matched_for_filtering {
    count(user_matched_policy_rule_for_filtering) > 0
}

# get all resources which matches the attributes
user_role_allowed_resources[resourceID] {
    some rule

    user_matched_role_rule_for_filtering[rule]
    res := data.resources[rule.resourceType][_]
    project_name_is_match(res)
    attributes_match(rule.matchAttributes, res)
    resourceID := res.resourceID
}

user_policy_allowed_resources[resourceID] {
    some rule

    user_matched_policy_rule_for_filtering[rule]
    res := data.resources[rule.resourceType][_]
    project_name_is_match(res)
    attributes_match(rule.matchAttributes, res)
    resourceID := res.resourceID
}

attributes_match(attributes, res) {
    count(attributes) == 0
}

attributes_match(attributes, res) {
    attribute := attributes[_]
    attribute_match(attribute, res)
}

attribute_match(attribute, res) {
    res.spec[attribute.key] == attribute.value
}

project_name_is_match(res) {
    res.projectName != ""
    res.projectName == project_name
}

project_name_is_match(res) {
    res.projectName == ""
}

user_matched_policy_rule[rule] {
    some rule

    allowed_policy_attributive_rules[rule]

    glob.match(trim(rule.endpoint, "/"), ["/"], concat("/", input.parsed_path))

}

user_matched_role_rule[rule] {
    some rule

    allowed_role_attributive_rules[rule]

    glob.match(trim(rule.endpoint, "/"), ["/"], concat("/", input.parsed_path))

}

user_matched_policy_rule_for_filtering[rule] {
    some rule

    allowed_policy_attributive_rules[rule]
    rule.method == http_request.method
    glob.match(trim(rule.endpoint, "/"), ["/"], concat("/", input.parsed_path))
    not rule.idRegex
}

user_matched_role_rule_for_filtering[rule] {
    some rule

    allowed_role_attributive_rules[rule]
    rule.method == http_request.method
    glob.match(trim(rule.endpoint, "/"), ["/"], concat("/", input.parsed_path))
    not rule.idRegex
}


all_attributes_match(attributes, resourceType, resourceID) {
    res := data.resources[resourceType][_]
    res.resourceID == resourceID
    project_name_is_match(res)

    attributes_match(attributes, res)
}

attributes_match(attributes, res) {
    attribute := attributes[_]
    attribute_match(attribute, res)
}

attributes_mismatch(attributes, res) {
    attribute := attributes[_]
    attribute_mismatch(attribute, res)
}

attribute_mismatch(attribute, res) {
    res.spec[attribute.key] != attribute.value
}

attribute_mismatch(attribute, res) {
    not res.spec[attribute.key]
}

get_resource_id(idRegex) = id {
    output := regex.find_all_string_submatch_n(trim(idRegex, "/"), concat("/", input.parsed_path), -1)
    count(output) == 1
    count(output[0]) == 2
    id := output[0][1]
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

    role.name == "project-admin"
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
user_projects[project] {
    some i
    data.bindings.policy_bindings[i].uid == claims.uid
    project := data.bindings.policy_bindings[i].bindings[_].namespace
}

# get all projects which are visible by all users (the user name is "*")
user_projects[project] {
    some i
    data.bindings.role_bindings[i].uid == "*"
    project := data.bindings.role_bindings[i].bindings[_].namespace
}

user_projects[project] {
    some i
    data.bindings.policy_bindings[i].uid == "*"
    project := data.bindings.policy_bindings[i].bindings[_].namespace
}

# all projects which are allowed by current user
user_allowed_projects[project] {
    some project
    user_projects[project]
    not user_is_admin
    response.allowed with project_name as project
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
    some j
    data.bindings.role_bindings[i].uid == claims.uid
    data.bindings.role_bindings[i].bindings[j].namespace == project_name
    role_ref := data.bindings.role_bindings[i].bindings[j].role_refs[_]
}

# if the proejct is visible by all users (the user name is "*"), the bound roles are also allowed
allowed_roles[role_ref] {
    some i
    some j
    data.bindings.role_bindings[i].uid == "*"
    data.bindings.role_bindings[i].bindings[j].namespace == project_name
    role_ref := data.bindings.role_bindings[i].bindings[j].role_refs[_]
}

allowed_policies[policy_ref] {
    some i
    some j
    data.bindings.policy_bindings[i].uid == claims.uid
    data.bindings.policy_bindings[i].bindings[j].namespace == project_name
    policy_ref := data.bindings.policy_bindings[i].bindings[j].policy_refs[_]
}

allowed_policies[policy_ref] {
    some i
    some j
    data.bindings.policy_bindings[i].uid == "*"
    data.bindings.policy_bindings[i].bindings[j].namespace == project_name
    policy_ref := data.bindings.policy_bindings[i].bindings[j].policy_refs[_]
}

allowed_role_rules[rule] {
    some role_ref
    allowed_roles[role_ref]

    some i
    data.roles.roles[i].name == role_ref.name
    data.roles.roles[i].namespace == role_ref.namespace
    rule := data.roles.roles[i].rules[_]
}

allowed_policy_rules[rule] {
    some policy_ref
    allowed_policies[policy_ref]

    some i
    data.policies.policies[i].name == policy_ref.name
    data.policies.policies[i].namespace == policy_ref.namespace
    rule := data.policies.policies[i].rules[_]
}

allowed_policy_plain_rules[rule] {
    rule := allowed_policy_rules[_]
    not rule.matchAttributes
    not rule.matchExpressions
}

allowed_role_plain_rules[rule] {
    rule := allowed_role_rules[_]
    not rule.matchAttributes
    not rule.matchExpressions
}

allowed_policy_attributive_rules[rule] {
    rule := allowed_policy_rules[_]
    rule.matchAttributes
}

allowed_role_attributive_rules[rule] {
    rule := allowed_role_rules[_]
    rule.matchAttributes
}
allowed_policy_attributive_rules[rule] {
    rule := allowed_policy_rules[_]
    rule.matchExpressions
}

allowed_role_attributive_rules[rule] {
    rule := allowed_role_rules[_]
    rule.matchExpressions
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
