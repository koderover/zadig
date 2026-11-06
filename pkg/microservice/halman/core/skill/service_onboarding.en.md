# Service initialization

## Applicable scenarios

The user wants to add one or more services to an existing K8s YAML or Helm project.

## Required information

Target project and its type, each service's name, existing configuration source, file grouping and required template parameters. Select an existing build template if a build is needed.

## Steps

1. Query the existing project type, service list, and configurations and templates accessible to the current user. Prevent name collisions and type mismatches.
2. Include at most twenty services per batch. Confirm each source, name and file grouping. Do not split a service's multiple YAML files into separate services.
3. Use only existing YAML, Charts or templates. Do not clone or write repositories. If only source code exists without usable deployment configuration, explain what is missing and ask for it.
4. Create builds only from existing build templates; skip builds if no suitable template exists. Reuse the actual configuration logic of `AutoCreateWorkflow` when workflows are needed.
5. Use the service initialization tool to prepare a batch operation plan. The program executes each item only after explicit authorization through the confirmation card.

## Clarification

Prefer the exact full template name supplied by the user; do not ask for IDs. For YAML and build templates, pass the full name as `query_catalog.keyword`. If unmatched, follow `next_page` until an exact match is found, then read its details. Ask about unclear or ambiguous names instead of substituting similar names. Other resources use the existing interfaces' name lookup. Do not conclude a resource is absent because the first page has no match.

List missing fields by service and preserve information already supplied for other services. Read the saved draft before applying changes to an existing task.

When the user specifies a count, such as three services, record `expected_service_count` on the first call even if service names are not yet known. Save an incomplete draft through the initialization tool without inventing placeholders. Confirm the count and names first. Ask once whether code host, repository and branch are shared, apply shared values to each service, then collect individual paths, templates and variables.

Continue the task using its original `skill_execution_id`. The backend restores and merges the draft; submit only changed parameters and the relevant service names instead of repeating the entire list. Omitting services or sending an empty list does not delete existing entries. Use `remove_services` only for explicit removal. Renaming requires explicitly removing the old name and adding the new one. Explicitly update `expected_service_count` when the count changes; omissions must not shrink the scope. Pass `false` explicitly to disable a boolean option, and `null` to clear optional configuration.

Group missing template parameters by service using the tool's returned fields. Do not ask again for supplied values. After a template change, recheck variables for that service without affecting others. Credentials and YAML-type parameters must be preconfigured in the original template; do not request them in chat.

## Output requirements

Use the system language selected for this generation to show service names, sources, grouping, templates and optional steps. Report success, failure or unknown status for each item. Do not recreate successful items; retrying failed items requires confirmation of a new plan.

## Completion criteria

Report batch completion only when all services in the confirmed scope have been created successfully. Accurately explain partial failures, a plan that has only been prepared, or unknown results.
