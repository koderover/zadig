use `%s`;

# 4.2.0 deprecated edit_release_plan action and replaced it with 3 granular actions:
# - edit_release_plan_metadata
# - edit_release_plan_approval
# - edit_release_plan_subtasks

# Insert the 3 new actions (if they don't exist)
INSERT INTO `action` (name, action, resource, scope)
VALUES
    ("编辑元数据", "edit_release_plan_metadata", "ReleasePlan", 2),
    ("编辑审批流程", "edit_release_plan_approval", "ReleasePlan", 2),
    ("编辑子任务", "edit_release_plan_subtasks", "ReleasePlan", 2)
ON DUPLICATE KEY UPDATE id=id;

# Migrate role_action_binding - for each role that had edit_release_plan, add the 3 new actions
INSERT IGNORE INTO `role_action_binding` (action_id, role_id)
SELECT new_action.id, old_binding.role_id
FROM `action` new_action
CROSS JOIN (
    SELECT DISTINCT rab.role_id
    FROM `role_action_binding` rab
    INNER JOIN `action` a ON rab.action_id = a.id
    WHERE a.action = 'edit_release_plan'
) old_binding
WHERE new_action.action IN ('edit_release_plan_metadata', 'edit_release_plan_approval', 'edit_release_plan_subtasks');

# Migrate role_template_action_binding - same logic for role templates
INSERT IGNORE INTO `role_template_action_binding` (action_id, role_template_id)
SELECT new_action.id, old_binding.role_template_id
FROM `action` new_action
CROSS JOIN (
    SELECT DISTINCT rtab.role_template_id
    FROM `role_template_action_binding` rtab
    INNER JOIN `action` a ON rtab.action_id = a.id
    WHERE a.action = 'edit_release_plan'
) old_binding
WHERE new_action.action IN ('edit_release_plan_metadata', 'edit_release_plan_approval', 'edit_release_plan_subtasks');

# Step 4: Delete the old action (cascades to delete old bindings via ON DELETE CASCADE)
DELETE FROM `action` WHERE action = "edit_release_plan";