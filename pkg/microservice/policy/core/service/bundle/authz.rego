package rbac

# By default, deny requests.
default allow = false

# Allow admins to do anything.
allow {
    user_is_admin
}

# Allow the action if the user is granted permission to perform the action.
allow {
    some grant
    user_is_granted[grant]

    grant.method == input.method
    grant.endpoint == input.endpoint
}

# Allow admins to do anything.
user_is_admin {
    some role
    roles[role]

    role.name == "admin"
    role.namespace == ""
}

roles[role] {
    some i
    data.role_bindings[i].user == input.user
    role_refs := data.role_bindings[i].role_refs
    role := role_refs[_]
}

user_is_granted[grant] {
    some role_ref
    roles[role_ref]

    some i
    data.roles[i].name == role_ref.name
    data.roles[i].namespace == role_ref.namespace
    grant := data.roles[i].rules[_]
}
