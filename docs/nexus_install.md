# Nexus OSS Installation for Local Testing

## Quick Start

```bash
# Start Nexus Repository Manager OSS
docker run -d \
  --name nexus \
  -p 8081:8081 \
  -v nexus-data:/nexus-data \
  sonatype/nexus3:3.72.0
```

## Access

- Web UI: http://localhost:8081
- Admin password: `docker exec nexus cat /nexus-data/admin.password`
- Default login: `admin` / password from file

## Configure Maven Repository

1. Log in to Nexus as admin
2. Go to **Administration** → **Repositories**
3. Click **Create repository**
4. Select **maven2 (hosted)**
5. Configure:
   - Name: `maven-releases`
   - Version policy: `Release`
6. Click **Create repository**

## Upload Test Artifacts (example with things-service)

Maven coordinates from `example/things/build/service/pom.xml`:
- groupId: `com.abatalev.demo`
- artifactId: `things-service`
- version: `0.0.1`

### Maven deploy

```bash
mvn deploy:deploy-file \
  -Durl=http://localhost:8081/repository/maven-releases \
  -DrepositoryId=nexus-releases \
  -Dfile=target/things-service-0.0.1.jar \
  -DgroupId=com.abatalev.demo \
  -DartifactId=things-service \
  -Dversion=0.0.1 \
  -Dpackaging=jar

## Configuration Example

```yaml
config:
  group_id: "com.abatalev.demo"

services:
  - id: things-service
    name: "Things Service"
    repository: "maven-releases"
    artifact_id: "things-service"
    sources_url_pattern: "{{repository}}/{{groupPath}}/{{artifactId}}/{{version}}/{{artifactId}}-{{version}}{{suffix}}.jar"
    versions:
      - version: "0.0.1"
```

URLs generated:
- Classes: `maven-releases/com/abatalev/demo/things-service/0.0.1/things-service-0.0.1.jar`
- Sources: `maven-releases/com/abatalev/demo/things-service/0.0.1/things-service-0.0.1-sources.jar`

Placeholders:
- `{{groupId}}` - original groupId (e.g., `com.abatalev.demo`)
- `{{groupPath}}` - groupId with `.` replaced by `/` (e.g., `com/abatalev/demo`)

## Maven deploy

```bash
cd example/things/build/service
mvn deploy -DskipTests -s .mvn/settings.xml
```

Note: Requires `distributionManagement` in pom.xml and credentials in `.mvn/settings.xml`.

## Stop and Remove

```bash
# Stop
docker stop nexus

# Remove (data will be lost)
docker rm nexus
docker volume rm nexus-data
```
