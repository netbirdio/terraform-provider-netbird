-- Seed Accounts table
INSERT INTO accounts (id, created_by, created_at, domain, domain_category, is_domain_primary_account, network_identifier, network_serial, settings_peer_login_expiration_enabled, settings_peer_login_expiration, settings_regular_users_view_blocked, settings_groups_propagation_enabled, settings_jwt_groups_enabled, settings_extra_peer_approval_enabled, network_net)
VALUES ('account1', 'user1', '2024-04-17 09:35:50.651027026+00:00', 'netbird.selfhosted', 'private', 1, 'network1', 694, 1, 86400000000000, 1, 1, 0, 0, '{"IP":"100.64.0.0","Mask":"//8AAA=="}');

-- Seed Users table
INSERT INTO users (id, account_id, role, is_service_user, non_deletable, blocked, created_at, issued)
VALUES ('user1', 'account1', 'owner', 0, 0, 0, '2024-08-12 00:00:00', 'api');

-- Seed Groups table
INSERT INTO groups (id, account_id, name, issued, integration_ref_id, integration_ref_integration_type)
VALUES ('group-all', 'account1', 'All', 'api',  0, NULL);
INSERT INTO groups (id, account_id, name, issued, integration_ref_id, integration_ref_integration_type)
VALUES ('group-notall', 'account1', 'NotAll', 'api',  0, NULL);

-- Seed Personal Access Tokens (API Keys) table
INSERT INTO personal_access_tokens (id, user_id, name, hashed_token, expiration_date, created_by, created_at, last_used)
VALUES ('1', 'user1', 'Test API Key', 'smJvzexPcQ3NRezrVDUmF++0XqvFvXzx8Rsn2y9r1z0=', '2124-08-12 00:00:00', 'user1', '2024-08-12 00:00:00', NULL);

-- Seed Peers
INSERT INTO peers (`id`,`account_id`,`key`,`ip`,`meta_hostname`,`meta_go_os`,`meta_kernel`,`meta_core`,`meta_platform`,`meta_os`,`meta_os_version`,`meta_wt_version`,`meta_ui_version`,`meta_kernel_version`,`meta_network_addresses`,`meta_system_serial_number`,`meta_system_product_name`,`meta_system_manufacturer`,`meta_environment`,`meta_files`,`name`,`dns_label`,`peer_status_last_seen` ,`peer_status_connected` ,`peer_status_login_expired` ,`peer_status_requires_approval` ,`user_id`,`ssh_key`,`ssh_enabled` ,`login_expiration_enabled` ,`last_login` ,`created_at` ,`ephemeral` ,`location_connection_ip`,`location_country_code`,`location_city_name`,`location_geo_name_id` )
VALUES ('peer1','account1','5rvhvriKJZ3S9oxYToVj5TzDM9u9y8cxg7htIMWlYAg=','"100.64.114.31"','f2a34f6a4731','linux','Linux','11','unknown','Debian GNU/Linux','','0.12.0','','',NULL,'','','','{"Cloud":"","Platform":""}',NULL,'peer1','peer1','2023-03-02 09:21:02.189035775+01:00',0,0,0,'','ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAILzUUSYG/LGnV8zarb2SGN+tib/PZ+M7cL4WtTzUrTpk',0,0,'2023-03-01 19:48:19.817799698+01:00','2024-10-02 17:00:32.527947+02:00',0,'""','','',0);

INSERT INTO peers (`id`,`account_id`,`key`,`ip`,`meta_hostname`,`meta_go_os`,`meta_kernel`,`meta_core`,`meta_platform`,`meta_os`,`meta_os_version`,`meta_wt_version`,`meta_ui_version`,`meta_kernel_version`,`meta_network_addresses`,`meta_system_serial_number`,`meta_system_product_name`,`meta_system_manufacturer`,`meta_environment`,`meta_files`,`name`,`dns_label`,`peer_status_last_seen` ,`peer_status_connected` ,`peer_status_login_expired` ,`peer_status_requires_approval` ,`user_id`,`ssh_key`,`ssh_enabled` ,`login_expiration_enabled` ,`last_login` ,`created_at` ,`ephemeral` ,`location_connection_ip`,`location_country_code`,`location_city_name`,`location_geo_name_id` )
VALUES ('peer2','account1','5rvhvriKJZ3S9oxYToVj5TzDM9u9y8cxg7htIMWlYAg=','"100.64.114.32"','f2a34f6a4731','linux','Linux','11','unknown','Debian GNU/Linux','','0.12.0','','',NULL,'','','','{"Cloud":"","Platform":""}',NULL,'peer2','peer2','2023-03-02 09:21:02.189035775+01:00',0,0,0,'','ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAILzUUSYG/LGnV8zarb2SGN+tib/PZ+M7cL4WtTzUrTpk',0,0,'2023-03-01 19:48:19.817799698+01:00','2024-10-02 17:00:32.527947+02:00',0,'""','','',0);

INSERT INTO peers (`id`,`account_id`,`key`,`ip`,`meta_hostname`,`meta_go_os`,`meta_kernel`,`meta_core`,`meta_platform`,`meta_os`,`meta_os_version`,`meta_wt_version`,`meta_ui_version`,`meta_kernel_version`,`meta_network_addresses`,`meta_system_serial_number`,`meta_system_product_name`,`meta_system_manufacturer`,`meta_environment`,`meta_files`,`name`,`dns_label`,`peer_status_last_seen` ,`peer_status_connected` ,`peer_status_login_expired` ,`peer_status_requires_approval` ,`user_id`,`ssh_key`,`ssh_enabled` ,`login_expiration_enabled` ,`last_login` ,`created_at` ,`ephemeral` ,`location_connection_ip`,`location_country_code`,`location_city_name`,`location_geo_name_id` )
VALUES ('peer3','account1','5rvhvriKJZ3S9oxYToVj5TzDM9u9y8cxg7htIMWlYAg=','"100.64.114.33"','f2a34f6a4731','darwin','MacOS','11','unknown','MacOS Sierra','','0.12.0','','',NULL,'','','','{"Cloud":"","Platform":""}',NULL,'peer3','peer3','2023-03-02 09:21:02.189035775+01:00',0,0,0,'','ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAILzUUSYG/LGnV8zarb2SGN+tib/PZ+M7cL4WtTzUrTpk',0,0,'2023-03-01 19:48:19.817799698+01:00','2024-10-02 17:00:32.527947+02:00',0,'""','','',0);

-- Seed Networks
INSERT INTO networks (`id`,`account_id`,`name`,`description`)
VALUES ('network1', 'account1', 'tfaccnetwork', '');

-- Seed Resources
INSERT INTO network_resources (`id`,`network_id`,`account_id`,`name`,`description`,`type`,`domain`,`prefix`,`enabled`)
VALUES ('resource1', 'network1', 'account1', 'resource1', '', 'domain', 'mock1.com', '', 1);
INSERT INTO network_resources (`id`,`network_id`,`account_id`,`name`,`description`,`type`,`domain`,`prefix`,`enabled`)
VALUES ('resource2', 'network1', 'account1', 'resource2', '', 'subnet', '', '"192.168.0.0/16"', 1);
INSERT INTO network_resources (`id`,`network_id`,`account_id`,`name`,`description`,`type`,`domain`,`prefix`,`enabled`)
VALUES ('resource3', 'network1', 'account1', 'resource3', '', 'host', '', '"10.0.0.5/32"', 1);
