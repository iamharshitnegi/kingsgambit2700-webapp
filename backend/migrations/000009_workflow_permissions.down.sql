DELETE FROM role_permission_defaults
WHERE permission_code IN (
    'billing.entitlement.view',
    'billing.entitlement.grant',
    'support.case.manage'
);

DELETE FROM permission_definitions
WHERE code IN (
    'billing.entitlement.view',
    'billing.entitlement.grant',
    'support.case.manage'
);
