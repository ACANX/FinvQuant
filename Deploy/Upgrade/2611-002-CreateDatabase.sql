-- =====================================================================
-- FinvQuant 一次性维护脚本：创建数据库与基础权限（需求 10881 / 10882）
--
-- 为什么不在 Deploy/Migrations/ 里自动执行：
--   迁移器（`internal/database/migrate.go`）把每个迁移脚本放在**单个事务**里执行，
--   而 PostgreSQL 的 `CREATE DATABASE` 不允许在事务块内运行
--   （`CREATE DATABASE cannot run inside a transaction block`）；
--   且迁移只能发生在**已连上的库**里（库不存在根本连不上）。
--   所以建库属于「部署前的一次性动作」，用 psql 手工执行本脚本；
--   迁移链里只保留 V100010__verify_database.sql 做只读校验。
--
-- 用法（连**任意已存在的库**执行，默认连 postgres 库）：
--   psql "postgres://postgres@127.0.0.1:5432/postgres" \
--        -v db=finv -v owner=finv \
--        -f 2611-002-CreateDatabase.sql
--
--   容器方式：PostgreSQL 镜像由 `POSTGRES_DB` 自动建库，一般**无需**执行本脚本；
--   裸机方式：`Install.sh` 会自动 `createdb`；仅在需要手工预置库/属主时执行。
--
-- 参数（psql 变量，缺省 db=finv、owner=finv）：
--   db     目标数据库名
--   owner  属主角色名（须已存在；本脚本**不建角色、不设密码**，
--          角色由迁移 V100001__role_finv_create.sql 或部署侧创建）
--
-- 幂等：
--   - 库已存在 → 跳过创建，只校正属主与授权；
--   - 重复执行不报错；不删除、不改动库内任何对象（无 DROP / TRUNCATE / ALTER ... SET SCHEMA）。
--
-- 执行后由服务启动时的迁移链接手：
--   V100001 角色 → V100002～V100009 建 schema → V100010～V100012 校验。
-- =====================================================================

\set ON_ERROR_STOP on

-- ---------------------------------------------------------------------
-- 0. 参数缺省值
-- ---------------------------------------------------------------------
\if :{?db}
\else
  \set db finv
\endif
\if :{?owner}
\else
  \set owner finv
\endif

\echo '>>> 建库参数：数据库 =' :db '，属主 =' :owner

-- ---------------------------------------------------------------------
-- 1. 前置校验：属主角色必须已存在（本脚本不建角色）
-- ---------------------------------------------------------------------
SELECT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = :'owner') AS fq_role_ok \gset
\if :fq_role_ok
\else
  \echo '!!! 建库中止：角色' :owner '不存在。请先创建角色（迁移 V100001，或超级用户执行 CREATE ROLE <name> LOGIN PASSWORD ...）'
  \quit 1
\endif

-- ---------------------------------------------------------------------
-- 2. 建库（已存在则跳过；CREATE DATABASE 不支持 IF NOT EXISTS，用 \gexec）
--    TEMPLATE template0 + ENCODING UTF8：不继承 template1 的编码与扩展
-- ---------------------------------------------------------------------
SELECT format('CREATE DATABASE %I OWNER %I ENCODING ''UTF8'' TEMPLATE template0', :'db', :'owner')
 WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = :'db')
\gexec

-- 属主校正（库已存在但属主不是目标角色时）
SELECT format('ALTER DATABASE %I OWNER TO %I', :'db', :'owner')
 WHERE EXISTS (
         SELECT 1
           FROM pg_database d
           JOIN pg_roles r ON r.oid = d.datdba
          WHERE d.datname = :'db'
            AND r.rolname <> :'owner'
       )
\gexec

-- ---------------------------------------------------------------------
-- 3. 切到目标库，补齐权限（与 Install.sh 的授权一致）
--    public schema 的 CREATE 权限是 V100011 / V100012 的校验项，缺了会被判失败
-- ---------------------------------------------------------------------
\connect :"db"

GRANT ALL ON DATABASE :"db" TO :"owner";
GRANT ALL ON SCHEMA public TO :"owner";

-- ---------------------------------------------------------------------
-- 4. 收敛校验（三项任一不符即以非 0 退出码结束）
-- ---------------------------------------------------------------------
SELECT (:'db'::text = current_database()) AS fq_db_ok \gset
\if :fq_db_ok
\else
  \echo '!!! 校验失败：当前连接的库不是目标库'
  \quit 1
\endif

SELECT EXISTS (
         SELECT 1 FROM pg_database WHERE datname = :'db' AND pg_get_userbyid(datdba) = :'owner'
       ) AS fq_owner_ok \gset
\if :fq_owner_ok
\else
  \echo '!!! 校验失败：数据库属主不是目标角色'
  \quit 1
\endif

SELECT has_schema_privilege(:'owner', 'public', 'CREATE') AS fq_priv_ok \gset
\if :fq_priv_ok
\else
  \echo '!!! 校验失败：角色对 schema public 无 CREATE 权限'
  \quit 1
\endif

\echo '<<< 完成：数据库' :db '已就绪（属主' :owner '，UTF8），可启动服务执行迁移'
