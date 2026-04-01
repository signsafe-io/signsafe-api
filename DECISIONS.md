# Architecture Decision Records

## 2026-03-27: HTTP 라우터 — chi 선택

**결정**: `github.com/go-chi/chi/v5`를 HTTP 라우터로 사용한다.

**이유**:
- 표준 `net/http` 핸들러 인터페이스를 완전히 준수하여 vendor lock-in 없음
- 미들웨어 체이닝, 그룹 라우팅, URL 파라미터 처리가 깔끔
- gin 대비 더 가볍고 표준 호환성이 높음
- 불필요한 추상화 없이 raw `http.Handler`를 그대로 사용 가능

**영향**: 없음 (내부 라우팅 결정)

---

## 2026-03-27: DB 접근 — sqlx 선택

**결정**: `github.com/jmoiron/sqlx`를 사용하며 ORM은 전혀 사용하지 않는다.

**이유**:
- raw SQL을 그대로 작성하여 DB 쿼리의 완전한 가시성 확보
- struct 태그(`db:"column_name"`)로 스캔 자동화 지원
- 아키텍처 원칙(ORM 금지) 준수

**영향**: 없음 (내부 데이터 레이어)

---

## 2026-03-27: ID 전략 — ULID 선택

**결정**: 모든 PK를 `VARCHAR(26)` ULID로 생성한다 (`github.com/oklog/ulid/v2`).

**이유**:
- UUID와 달리 시간순 정렬 가능 (lexicographic sort = created_at sort)
- URL-safe, 대소문자 구분 없음
- DB 인덱스 성능이 UUID v4보다 우수 (monotonic 삽입)

**영향**: signsafe-ai, signsafe-web이 contractId, analysisId 등을 26자 문자열로 기대해야 함

---

## 2026-03-27: 파일 업로드 전략 — 서버 프록시 방식

**결정**: 클라이언트가 signsafe-api에 직접 multipart 업로드하고, API 서버가 SeaweedFS로 스트리밍한다. Presigned URL은 다운로드 전용으로만 사용한다.

**이유**:
- 업로드 파일 크기 제한, 바이러스 검사, 메타데이터 저장을 서버에서 원자적으로 처리 가능
- 클라이언트가 스토리지 엔드포인트를 몰라도 됨

**영향**: 업로드 시 signsafe-api가 네트워크 프록시 역할을 함 (메모리 사용 주의)

---

## 2026-03-27: 토큰 전략 — JWT + Opaque Refresh Token

**결정**:
- Access Token: JWT (HMAC-SHA256), 1시간 만료, Authorization: Bearer 헤더
- Refresh Token: cryptographically random opaque token, SHA-256 해시를 DB 저장, 30일 만료
- Refresh Token은 Redis에도 캐시하여 검증 속도 향상

**이유**:
- Access Token을 stateless JWT로 유지하면 매 요청마다 DB 조회 불필요
- Refresh Token을 opaque로 하면 즉시 무효화 가능 (JWT는 만료 전 무효화 불가)

**영향**: signsafe-web은 Access Token을 메모리에, Refresh Token을 httpOnly Cookie에 저장해야 함

---

## 2026-03-30: 조직 생성 및 내 조직 목록 API 추가

**결정**:
- `GET /users/me/organizations`: 인증된 사용자가 속한 모든 조직과 각각의 role을 배열로 반환한다.
- `POST /organizations`: 이름을 받아 새 조직을 생성하고, 생성자를 admin으로 자동 등록한다.
- `userRepo.ListUserOrganizations`: organizations와 user_organizations를 JOIN하여 id, name, plan, role을 한 번에 조회한다.
- `userRepo.CreateOrganizationWithAdmin`: organizations INSERT와 user_organizations INSERT를 단일 트랜잭션으로 처리한다.
- 새로 생성되는 조직의 기본 plan은 `"free"`, features는 `"[]"`로 설정한다.

**이유**:
- 회원가입 시 자동 생성되는 개인 조직 외에 팀/프로젝트 단위로 추가 조직을 만들 수 있어야 함
- 다중 조직 멤버십을 가진 사용자가 조직 전환 UI를 구현하려면 전체 소속 목록이 필요함
- org + membership을 단일 트랜잭션으로 처리하여 orphan 조직(멤버 없는 조직)이 생기지 않도록 보장

**영향**: signsafe-web이 조직 선택/전환 UI를 구현할 때 `GET /users/me/organizations`를 사용해야 함

---

## 2026-03-27: 비동기 작업 패턴 — Job ID 즉시 반환

**결정**: 파싱, 분석 등 시간이 걸리는 모든 작업은 즉시 Job ID를 반환하고 RabbitMQ 큐에 위임한다.

**이유**:
- HTTP 타임아웃 방지
- 클라이언트 UX 향상 (진행률 폴링)
- 실패 시 DLQ에서 재시도 가능

**영향**: 클라이언트는 Job 상태를 폴링해야 함 (1-3초 간격 권장)

---

## 2026-03-31: 에러 처리 패턴 — sentinel error + errors.Is 통일

**결정**: 모든 서비스 레이어는 sentinel error를 `fmt.Errorf("ctx: %w", ErrXxx)` 패턴으로 wrap하여 반환하고, 핸들러는 `errors.Is`로 분기한다. 문자열 비교(`strings.Contains`, `err.Error() ==`) 패턴을 전면 제거한다.

추가된 sentinel: `ErrWrongPassword`, `ErrPasswordTooShort` (`service/errors.go`)

**이유**:
- 에러 메시지 문자열이 변경되어도 핸들러의 분기 로직이 깨지지 않음
- 래핑 깊이와 무관하게 `errors.Is`는 체인 전체를 탐색하므로 신뢰성 있음
- 테스트에서 에러 종류를 명확하게 단언 가능

**영향**: 없음 (HTTP 응답 코드 및 외부 API 동작은 동일)

---

## 2026-03-31: 중복 logAudit 메서드 제거

**결정**: `ContractHandler.logAudit`, `AnalysisHandler.logAudit` 두 개의 중복 메서드를 제거하고, `helpers.go`의 패키지 레벨 함수 `logAuditEvent`를 공통으로 사용한다.

**이유**:
- 동일한 IP 추출 + audit 이벤트 발행 로직이 3개 위치에 분산되어 있었음
- `audit_handler.go`의 인라인 IP 추출 코드도 `clientIP()` helper로 대체

**영향**: 없음 (동작 동일)

---

## 2026-04-01: 대시보드 만료 구간별 통계 추가

**결정**: `GET /organizations/{orgId}/stats` 응답에 `expiryBuckets { days30, days60, days90 }` 추가.

**이유**:
- 기존 `expiringSoon`(30일)만으로는 계약 갱신 우선순위 판단 부족
- 30/60/90일 구간을 단일 SQL 쿼리(`COUNT(*) FILTER (WHERE ...)` 3개)로 추가 → 쿼리 수 변동 없음
- 기존 `expiringSoon` 필드는 하위 호환을 위해 유지 (`= expiring30` 값)

**영향**: 마이그레이션 불필요 (DB 스키마 변경 없음, SQL 집계만 변경)

---

## 2026-04-01: confidence score 마이그레이션

**결정**: `clause_results.confidence FLOAT NOT NULL DEFAULT 0.5` 컬럼 추가 (migration 000006).

**이유**:
- signsafe-ai LLM 분석 결과에 신뢰도 0~1 포함
- 기존 행에는 DEFAULT 0.5 적용 (중립값)
- NOT NULL로 선언하여 API 응답에서 null 처리 불필요

**영향**: signsafe-ai `insert_clause_result` SQL 업데이트 필요 (완료)
