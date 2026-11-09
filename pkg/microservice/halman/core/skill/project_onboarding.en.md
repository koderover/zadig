# Project initialization

## Applicable scenarios

The user wants to create a K8s YAML or Helm project and initialize services using existing service configurations.

## Required information

Project display name, project key, project type, and each service's name and existing service configuration source. Collect code host, repository, branch, file grouping and template parameters as needed. Creating an environment also requires its name, cluster and other necessary parameters.

## Steps

1. Query existing code hosts, configurations and templates accessible to the current user. Resolve names to internal identifiers without asking the user to provide IDs.
2. Default to a private project with the current user as administrator. Confirm the project name, key and K8s YAML or Helm type.
3. Use existing YAML, Charts or service templates. Do not clone or write repositories, or generate service configurations or build scripts from source code.
4. Include at most twenty services per batch. Identify each source and file grouping. Do not automatically turn every YAML file into a separate service or overwrite resources with the same name.
5. Create builds only from existing build templates; explicitly skip builds if no suitable template exists. When workflows are needed, reuse `AutoCreateWorkflow` without changing its rules based on whether builds exist.
6. Use the project initialization tool to prepare a plan. Environment creation may deploy resources and must be included in the confirmation scope. The program executes the plan after confirmation through the card.

## Clarification

Prefer the exact full template name supplied by the user; do not ask for IDs. For YAML and build templates, pass the full name as `query_catalog.keyword`. If unmatched, follow `next_page` until an exact match is found, then read its details. Ask about unclear or ambiguous names instead of substituting similar names. Other resources use the existing interfaces' name lookup. Do not conclude a resource is absent because the first page has no match.

Ask about missing fields returned by the tool, prefer existing candidates and do not repeatedly ask for saved parameters. Restore the saved draft before merging changes to an existing task.

When the user specifies a count, such as three services, record `expected_service_count` on the first call even if service names are not yet known. Save an incomplete draft through the initialization tool without inventing placeholders. Confirm the count and names first. Ask once whether code host, repository and branch are shared, apply shared values to each service, then collect individual paths, templates and variables.

Continue the task using its original `skill_execution_id`. The backend restores and merges the draft; submit only changed parameters and the relevant service names instead of repeating the entire list. Omitting services or sending an empty list does not delete existing entries. Use `remove_services` only for explicit removal. Renaming requires explicitly removing the old name and adding the new one. Explicitly update `expected_service_count` when the count changes; omissions must not shrink the scope. Pass `false` explicitly to disable a boolean option, and `null` to clear optional configuration.

Group missing template parameters by service using the tool's returned fields. Do not ask again for supplied values. After a template change, recheck variables for that service without affecting others. Credentials and YAML-type parameters must be preconfigured in the original template; do not request them in chat.

## Output requirements

Use the system language selected for this generation to show the project, services, sources and planned actions. Explain why optional steps such as builds are skipped. Clearly distinguish missing information, pending confirmation and individual execution results. Do not request or display plaintext secrets.

## Completion criteria

Report completion only when all required steps in the confirmed scope have finished and optional steps have been skipped according to the rules. Missing fields, pending confirmation, failures and unknown results do not count as completion.
