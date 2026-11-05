-- Independent permissions for the first durable finance and support workflow
-- slices. Family-scoped guardian access remains relationship based.

INSERT INTO permission_definitions (code, description)
VALUES
    ('billing.entitlement.view', 'View entitlement balances and immutable ledger history'),
    ('billing.entitlement.grant', 'Grant session credits with audited financial authority'),
    ('support.case.manage', 'Assign and operate family support cases, including internal notes');

INSERT INTO role_permission_defaults (role, permission_code)
VALUES
    ('finance_admin', 'billing.entitlement.view'),
    ('finance_admin', 'billing.entitlement.grant'),
    ('operations_admin', 'support.case.manage'),
    ('super_admin', 'billing.entitlement.view'),
    ('super_admin', 'billing.entitlement.grant'),
    ('super_admin', 'support.case.manage');
