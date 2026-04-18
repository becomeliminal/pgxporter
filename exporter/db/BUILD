go_library(
    name = "db",
    srcs = [
        "db.go",
        "dsn.go",
        "opts.go",
        "pg_lock.go",
        "pg_stat_activity.go",
        "pg_stat_statements.go",
        "pg_stat_user_indexes.go",
        "pg_stat_user_tables.go",
        "pg_statio_user_indexes.go",
        "pg_statio_user_tables.go",
    ],
    visibility = ["PUBLIC"],
    deps = [
        "//exporter/db/model",
        "//exporter/logging",
        "//third_party/go:pgx.v5",
        "//third_party/go:scany.v2",
    ],
)
