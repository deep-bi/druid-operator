<!--
Licensed to the Apache Software Foundation (ASF) under one
or more contributor license agreements.  See the NOTICE file
distributed with this work for additional information
regarding copyright ownership.  The ASF licenses this file
to you under the Apache License, Version 2.0 (the
"License"); you may not use this file except in compliance
with the License.  You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing,
software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
KIND, either express or implied.  See the License for the
specific language governing permissions and limitations
under the License.
-->

# Deep.BI Druid Operator

## Overview

Deep.BI maintains a patched fork of the [Apache Druid Operator](https://github.com/apache/druid-operator) with bug fixes, improvements, and production-hardened defaults. All changes are designed to be fully compatible with the upstream operator. Fixes are contributed back upstream where applicable.

Releases are published here: [Deep.BI Druid Operator Releases](https://github.com/deep-bi/druid-operator/releases)

## Installation

The operator is distributed as a Helm chart hosted at `https://charts.deep.bi`.

```bash
helm repo add deep-bi https://charts.deep.bi
helm repo update
helm install druid-operator deep-bi/druid-operator -n druid-operator --create-namespace
```

To upgrade to a specific version:
```bash
helm upgrade druid-operator deep-bi/druid-operator --version 0.4.1 -n druid-operator
```

## Versioning and Compatibility

Each release is tagged as `v<version>` (e.g. `v1.3.2`) with a corresponding Helm chart version. The operator is a drop-in replacement for the upstream Apache Druid Operator.

Docker images are published to Docker Hub as [`deepbi/druid-operator`](https://hub.docker.com/r/deepbi/druid-operator).

## Documentation

- [API Specifications](docs/api_specifications/druid.md)
- [Features](docs/features.md)
- [Release Process](RELEASE.md)

## Support

For questions or commercial support, contact [druid@deep.bi](mailto:druid@deep.bi).
