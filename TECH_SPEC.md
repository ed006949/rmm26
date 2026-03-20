# Technical Specification: rmm26 (Remote Monitoring and Management)

## 1. Overview

Briefly describe the purpose of the rmm26 application.

## 2. Goals and Non-Goals

### 2.1 Goals

- Goal 1: Describe a primary objective.
- Goal 2: Describe another primary objective.

### 2.2 Non-Goals

- Non-Goal 1: Define what this app will NOT do.

## 3. Architecture

### 3.1 High-Level Architecture

- **Core Principle:** All communication flows through Redis using the `rueidis` client.
- **Data Persistence:** Primary data store is Redis, leveraging RedisJSON for structured data and RediSearch for
  querying.
- **Memory Management:** Minimal data is stored in RAM; the application remains largely stateless, relying on Redis for
  state and cache.
- **Caching:** Redis is used as the primary cache layer (via `rueidis` client-side caching or direct Redis commands).
- **Data Integration:** Other data sources are processed to/from Redis, acting as a central message bus and data hub.
- **LDAP-style Organization:** Data is organized hierarchically using a Distinguished Name (DN) structure, implemented
  as Redis keys or fields within RedisJSON.
    - **Entries:** Each node in the tree is an entry (RedisJSON document).
    - **Schemas:** Attributes are governed by objectClasses and attribute types, similar to LDAP.
    - **Indexing:** RediSearch is used to provide LDAP-like search capabilities (filtering by attributes, DN
      base/scope).

### 3.2 Tech Stack

- **Language:** Go
- **Database/Broker:** Redis (Modules: RediSearch, RedisJSON)
- **Redis Client:** [Rueidis](https://github.com/redis/rueidis)
- **Authentication:** LDAP (LDAP/v3)

## 4. Detailed Design

### 4.1 Data Models

Data is organized according to an LDAP-like structure.

- **Entry:** Represented as a RedisJSON document.
    - **Key:** `dn:<distinguishedName>` (e.g., `dn:uid=user1,ou=users,dc=example,dc=com`).
    - **Structure:**
      ```json
      {
        "dn": "uid=user1,ou=users,dc=example,dc=com",
        "objectClass": ["person", "inetOrgPerson"],
        "attributes": {
          "cn": ["User One"],
          "sn": ["One"],
          "mail": ["user1@example.com"]
        }
      }
      ```
- **Schema/ObjectClasses:** Define the allowed attributes for an entry.
- **Attributes:** Stored in a JSON object, each attribute being an array of values (to support multi-valued attributes).

- **Indexing (RediSearch):**
    - A RediSearch index (e.g., `idx:dn`) should be created over RedisJSON documents with the prefix `dn:`.
    - Important fields for indexing:
        - `$.dn` (TEXT/TAG)
        - `$.objectClass[*]`. (TAG)
        - Specific common attributes (e.g., `$.attributes.cn[*]`, `$.attributes.mail[*]`) as TAG or TEXT.
    - Example Search Command: `FT.SEARCH idx:dn "@objectClass:{person} @attributes.mail:{user1@example.com}"`

### 4.2 Standard LDAP Schema Support

To maintain compatibility and clear semantics, rmm26 follows standard LDAP schema definitions (OIDs and Syntaxes) where
possible.

#### 4.2.1 Common Attribute Syntaxes (Data Types)

| Syntax Name      | OID                             | Description                   | Redis Type          |
|:-----------------|:--------------------------------|:------------------------------|:--------------------|
| Boolean          | `1.3.6.1.4.1.1466.115.121.1.7`  | `TRUE` or `FALSE`             | `boolean`           |
| Integer          | `1.3.6.1.4.1.1466.115.121.1.27` | Integer numeric value         | `number`            |
| Directory String | `1.3.6.1.4.1.1466.115.121.1.15` | UTF-8 encoded string          | `string`            |
| Generalized Time | `1.3.6.1.4.1.1466.115.121.1.24` | ISO 8601-like timestamp       | `string` (ISO 8601) |
| Octet String     | `1.3.6.1.4.1.1466.115.121.1.40` | Arbitrary byte string         | `string` (Base64)   |
| UUID             | `1.3.6.1.1.16.1`                | Universally Unique Identifier | `string` (UUID)     |

#### 4.2.2 Common Attribute Types

| Name              | OID                          | Syntax           | Description                                  |
|:------------------|:-----------------------------|:-----------------|:---------------------------------------------|
| `objectClass`     | `2.5.4.0`                    | OID              | Defines the type of entry                    |
| `cn`              | `2.5.4.3`                    | Directory String | Common Name                                  |
| `sn`              | `2.5.4.4`                    | Directory String | Surname                                      |
| `uid`             | `0.9.2342.19200300.100.1.1`  | Directory String | Unique ID                                    |
| `mail`            | `0.9.2342.19200300.100.1.3`  | Directory String | RFC 822 Email Address                        |
| `ou`              | `2.5.4.11`                   | Directory String | Organizational Unit Name                     |
| `dc`              | `0.9.2342.19200300.100.1.25` | Directory String | Domain Component                             |
| `entryUUID`       | `1.3.6.1.1.16.4`             | UUID             | Operational: Unique identifier for the entry |
| `createTimestamp` | `2.5.18.1`                   | Generalized Time | Operational: Time entry was created          |

#### 4.2.3 Common Object Classes

| Name                 | OID                          | Type       | Superior               | Must          |
|:---------------------|:-----------------------------|:-----------|:-----------------------|:--------------|
| `top`                | `2.5.6.0`                    | Abstract   | -                      | `objectClass` |
| `person`             | `2.5.6.6`                    | Structural | `top`                  | `cn`, `sn`    |
| `organizationalUnit` | `2.5.6.5`                    | Structural | `top`                  | `ou`          |
| `domain`             | `0.9.2342.19200300.100.4.13` | Structural | `top`                  | `dc`          |
| `inetOrgPerson`      | `2.16.840.1.113730.3.2.2`    | Structural | `organizationalPerson` | -             |

### 4.3 Application-Specific Schemas

These schemas define the data models specific to the rmm26 application, such as daemon configurations and network
equipment types.

#### 4.3.1 Custom Attribute Types

| Name         | OID                        | Syntax           | Description                                |
|:-------------|:---------------------------|:-----------------|:-------------------------------------------|
| `configData` | `1.3.6.1.4.1.99999.1.1`    | Directory String | JSON-encoded configuration data for daemon |
| `ipAddress`  | `1.3.6.1.4.1.1466.115.1.4` | Directory String | IP Address (IPv4 or IPv6)                  |
| `modelName`  | `1.3.6.1.4.1.99999.1.2`    | Directory String | Hardware model name                        |
| `capacity`   | `1.3.6.1.4.1.99999.1.3`    | Integer          | Capacity/Throughput in Gbps                |

#### 4.3.2 Custom Object Classes

| Name           | OID                     | Type       | Superior | Must               | May                        |
|:---------------|:------------------------|:-----------|:---------|:-------------------|:---------------------------|
| `daemonConfig` | `1.3.6.1.4.1.99999.2.1` | Structural | `top`    | `cn`, `configData` | `description`              |
| `router`       | `1.3.6.1.4.1.99999.2.2` | Structural | `top`    | `cn`, `ipAddress`  | `modelName`, `description` |
| `bigRouter`    | `1.3.6.1.4.1.99999.2.3` | Structural | `router` | `capacity`         | -                          |

### 4.4 APIs

Define internal/external APIs or interfaces.

- **Endpoint:** Description, input, output.

### 4.5 Components

- **Component 1:** Role and responsibility.

## 5. Security Considerations

- Authentication mechanism (LDAP).
- Authorization and Access Control.
- Data Encryption.

## 6. Performance and Scalability

- Expected load.
- Resource usage.
- Performance targets.

## 7. Future Enhancements

- Planned features for future iterations.

## 8. Timeline and Milestones

- **Milestone 1:** Description and date.
- **Milestone 2:** Description and date.
