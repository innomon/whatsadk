## Extract contacts

docker exec -i whatsadk-db psql -U postgres -d whatsadk -c "SELECT * FROM whatsmeow_contacts;" --expanded >> whatsapp-contacts.txt


docker start whatsadk-db


---

## List text/images for a number



---

## List Tables
docker exec -i whatsadk-db psql -U postgres -d whatsadk -c "\dt"
                        List of tables
 Schema |               Name                | Type  |  Owner   
--------+-----------------------------------+-------+----------
 public | blacklisted_numbers               | table | postgres
 public | filesys                           | table | postgres
 public | whatsmeow_app_state_mutation_macs | table | postgres
 public | whatsmeow_app_state_sync_keys     | table | postgres
 public | whatsmeow_app_state_version       | table | postgres
 public | whatsmeow_chat_settings           | table | postgres
 public | whatsmeow_commands                | table | postgres
 public | whatsmeow_contacts                | table | postgres
 public | whatsmeow_device                  | table | postgres
 public | whatsmeow_event_buffer            | table | postgres
 public | whatsmeow_identity_keys           | table | postgres
 public | whatsmeow_lid_map                 | table | postgres
 public | whatsmeow_message_secrets         | table | postgres
 public | whatsmeow_pre_keys                | table | postgres
 public | whatsmeow_privacy_tokens          | table | postgres
 public | whatsmeow_retry_buffer            | table | postgres
 public | whatsmeow_sender_keys             | table | postgres
 public | whatsmeow_sessions                | table | postgres
 public | whatsmeow_version                 | table | postgres

---

docker exec -i whatsadk-db psql -U postgres -d whatsadk -c "SELECT * FROM whatsmeow_sessions;" --expanded >> whatsmeow_sessions.txt

## Rec count

  ### 1. Check if there are any records matching that number anywhere in the path                                                                                                          
                                                                                                                                                                                           
  This query uses wildcards to see if the number exists with a country-code prefix (like  917838534799 ):                                                                                  
                                                                                                                                                                                           
    docker exec -i whatsadk-db psql -U postgres -d whatsadk -c "SELECT count(*) FROM filesys WHERE path LIKE '%7838534799%';"                                                              
                                                                                                                                                                                           
  ### 2. List all unique phone numbers stored in the database
  
  This query extracts all unique user prefixes from the paths to show you exactly how the directories are named:
  
    docker exec -i whatsadk-db psql -U postgres -d whatsadk -c "SELECT DISTINCT split_part(path, '/', 2) AS user_id FROM filesys WHERE path LIKE 'whatsmeow/%';"
  
  ### 3. Check the total number of records in the filesys table
  
    docker exec -i whatsadk-db psql -U postgres -d whatsadk -c "SELECT count(*) FROM filesys;"



