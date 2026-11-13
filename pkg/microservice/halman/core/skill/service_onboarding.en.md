# Service initialization

## Applicable scenarios

The user wants to add services to an existing K8s YAML or Helm project using existing YAML, Helm Charts or service templates, or create builds for services already in Zadig.

## Required information

Target project and its type, each new service's name, the source of its YAML, Helm Chart or service template, file grouping and required template parameters. Select an existing build template if a build is needed.

## Steps

If the user has not named another project and the current page identifies a project, use it for queries and plan preparation and state the target in the response; do not ask for a separate project confirmation. Explicit user targets and saved drafts take precedence. Ask only when no page project is available or targets conflict. Preparing a plan does not execute writes; the confirmation card still authorizes execution. The service initialization tool reads the existing project's type.

1. The service initialization tool checks the existing project type and service name conflicts. Use catalog tools for configurations and templates accessible to the current user. Prevent name collisions and type mismatches.
2. Include at most twenty services per batch. Confirm each source, name and file grouping. Do not split a service's multiple YAML files into separate services.
3. Use only existing YAML, Charts or templates. Do not clone or write repositories. If only source code exists without usable service configuration, explain what is missing and ask for it.
4. Create builds only from existing build templates; skip builds if no suitable template exists. Saying "only use existing build templates" constrains how builds are created; it does not request builds. Do not ask for build details unless the user explicitly requests builds. Service initialization does not create workflows. Do not ask about workflows or list them as skipped in responses or plans. Omit `create_workflow` or set it to false.
5. Use the service initialization tool to prepare a batch operation plan. The program executes each item only after explicit authorization through the confirmation card.

For Chart repository imports, pass the target `project_key` when querying `helm_repositories` and select the repository by `repo_name`. For `chart_versions`, pass the repository name as `repo_name` (or `id`), never its internal ID. The result contains versions for every Chart in that repository: query once and reuse it across the batch. Preserve the returned version order; when the user requests the latest version, select each Chart's first entry and show the concrete version in the plan. Set each service's `chart_repo` to the repository name as well. Do not ask for code hosts, source repositories or branches unless Git configuration or builds are needed.

For builds targeting existing services, use `builds` directly; `services` may be omitted or contain only their names. Read the project type and service components from Zadig. Never ask again for YAML/Chart sources. Auto-select a sole component, and ask the user to choose if multiple components exist. Collect only build names, the existing build template, source repositories/branches and required variables. Do not create services or environments unless explicitly requested. A new build request after service initialization is a new task: do not reuse the completed initialization draft or its service sources.

## Clarification

For Git configurations, use only file or directory paths explicitly supplied by the user, including paths already saved in the original task draft. If the repository is known but the path is missing, save the known parameters and ask for the path. Do not search the repository, browse directories, guess common paths, reinterpret template names as paths, or call `configuration_paths` to explore. Query only the supplied path without expanding to parent directories or other paths. Do not request Git paths for template-library or Chart-repository sources.

The tool collects service name conflicts, saves the draft, and returns them as `awaiting_input`. Explain those conflicts and ask how to resolve them; do not query the project catalog or change the service list to probe for more conflicts. Never remove services without an explicit request. Chart repository imports require the service name to match the Chart name: renaming only the service cannot resolve a conflict. Ask whether to remove conflicting services from this batch or select other Charts, preserving existing project services.

Prefer the exact full template name supplied by the user; do not ask for IDs. For YAML and build templates, pass the full name as `query_catalog.keyword`. If unmatched, follow `next_page` until an exact match is found, then read its details. Ask about unclear or ambiguous names instead of substituting similar names. Other resources use the existing interfaces' name lookup. Do not conclude a resource is absent because the first page has no match.

List missing fields by service and preserve information already supplied for other services. Read the saved draft before applying changes to an existing task.

When the user specifies a count, such as three services, record `expected_service_count` on the first call even if service names are not yet known. Save an incomplete draft through the initialization tool without inventing placeholders. Confirm the count and names first. Ask once whether code host, repository and branch are shared, apply shared values to each service, then collect individual paths, templates and variables.

Continue the task using its original `skill_execution_id`. The backend restores and merges the draft; submit only changed parameters and the relevant service names instead of repeating the entire list. Omitting services or sending an empty list does not delete existing entries. Use `remove_services` only for explicit removal. Renaming requires explicitly removing the old name and adding the new one. Explicitly update `expected_service_count` when the count changes; omissions must not shrink the scope. Pass `false` explicitly to disable a boolean option, and `null` to clear optional configuration.

Group missing template parameters by service using the tool's returned fields. Do not ask again for supplied values. After a template change, recheck variables for that service without affecting others. Unchanged variables inherit template values. Submit only overrides explicitly requested by the user; credential variables support overrides too. Do not copy template defaults back as overrides or submit redaction placeholders. Do not echo credential values in replies or confirmation descriptions. YAML-type parameters must still be preconfigured in the original template.

When collecting additional information, query only resources needed by newly supplied or changed fields and their dependencies. Do not repeat catalog queries for unchanged templates, code sources or resolved parameters. Submit the changes directly to the business tool; the backend reuses draft validation results and collects missing fields. The program rechecks permissions and template changes before execution.

## Output requirements

Use the system language selected for this generation to show service names, sources, grouping, templates and optional steps. Report success, failure or unknown status for each item. Do not recreate successful items; retrying failed items requires confirmation of a new plan.

## Completion criteria

Report batch completion only when all operations in the confirmed scope have succeeded (including builds when the request is build-only). Accurately explain partial failures, a plan that has only been prepared, or unknown results.
