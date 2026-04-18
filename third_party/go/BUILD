package(default_visibility = ["PUBLIC"])

go_toolchain(
    name = "gotool",
    strip_srcs = True,
    version = "1.23.4",
)

go_module(
    name = "golint",
    binary = True,
    download = ":x-lint-download",
    install = ["golint"],
    module = "golang.org/x/lint",
    deps = [":x_tools"],
)

go_mod_download(
    name = "x-lint-download",
    module = "golang.org/x/lint",
    version = "v0.0.0-20210508222113-6edffad5e616",
    deps = [":x_tools"],
)

go_module(
    name = "x_tools",
    install = ["..."],
    module = "golang.org/x/tools",
    version = "v0.1.4",
    deps = [
        ":goldmark",
        ":x_crypto",
        ":x_errors",
        ":x_mod",
        ":x_net",
        ":x_sync",
    ],
)

go_module(
    name = "x_term",
    module = "golang.org/x/term",
    version = "v0.0.0-20210615171337-6886f2dfbf5b",
    deps = [":x_sys"],
)

go_module(
    name = "x_crypto",
    install = ["..."],
    module = "golang.org/x/crypto",
    strip = [
        "acme/autocert",
    ],
    version = "v0.0.0-20210616213533-5ff15b29337e",
    deps = [
        ":x_sys",
        ":x_term",
    ],
)

go_module(
    name = "x_sys",
    install = ["..."],
    module = "golang.org/x/sys",
    version = "v0.0.0-20210415045647-66c3f260301c",
)

go_module(
    name = "x_sync",
    install = ["..."],
    module = "golang.org/x/sync",
    version = "v0.0.0-20210220032951-036812b2e83c",
)

go_module(
    name = "x_net",
    install = ["..."],
    module = "golang.org/x/net",
    strip = ["http2/h2demo"],
    version = "v0.0.0-20210614182718-04defd469f4e",
    deps = [
        ":x_crypto",
        ":x_text",
    ],
)

go_module(
    name = "x_text",
    install = ["..."],
    module = "golang.org/x/text",
    strip = [
        "cmd",
        "message/pipeline",
    ],
    version = "v0.3.6",
)

go_module(
    name = "x_mod",
    install = ["..."],
    module = "golang.org/x/mod",
    version = "v0.5.1",
    deps = [
        ":x_crypto",
        ":x_errors",
    ],
)

go_module(
    name = "x_errors",
    install = ["..."],
    module = "golang.org/x/xerrors",
    version = "v0.0.0-20200804184101-5ec99f83aff1",
)

go_module(
    name = "goldmark",
    install = ["..."],
    module = "github.com/yuin/goldmark",
    version = "v1.3.9",
)

go_module(
    name = "prometheus-client",
    install = ["..."],
    module = "github.com/prometheus/client_golang",
    version = "v1.0.0",
    deps = [
        ":client_model",
        ":json-iterator",
        ":perks",
        ":procfs",
        ":prometheus-common",
        ":protobuf",
        ":x_net",
    ],
)

go_module(
    name = "client_model",
    install = ["..."],
    module = "github.com/prometheus/client_model",
    version = "v0.2.0",
    deps = [
        ":protobuf-v1",
    ],
)

go_module(
    name = "protobuf-v1",
    download = ":protobuf-v1-download",
    install = ["..."],
    module = "github.com/golang/protobuf",
    deps = [
        ":protobuf",
    ],
)

go_module(
    name = "protobuf",
    download = ":protobuf-download",
    install = ["..."],
    module = "google.golang.org/protobuf",
    deps = [":cmp"],
)

go_mod_download(
    name = "protobuf-download",
    module = "google.golang.org/protobuf",
    version = "v1.27.1",
)

go_module(
    name = "cmp",
    install = [
        "cmp",
        "cmp/cmpopts",
    ],
    module = "github.com/google/go-cmp",
    version = "v0.5.6",
)

go_mod_download(
    name = "protobuf-v1-download",
    module = "github.com/golang/protobuf",
    version = "v1.5.2",
)

go_module(
    name = "json-iterator",
    install = ["..."],
    module = "github.com/json-iterator/go",
    version = "v1.1.6",
    deps = [
        ":concurrent",
        ":reflect2",
    ],
)

go_module(
    name = "concurrent",
    install = ["..."],
    module = "github.com/modern-go/concurrent",
    version = "1.0.3",
)

go_module(
    name = "reflect2",
    install = ["..."],
    module = "github.com/modern-go/reflect2",
    version = "v1.0.1",
    deps = [
        ":concurrent",
    ],
)

go_module(
    name = "perks",
    install = ["..."],
    module = "github.com/beorn7/perks",
    version = "v1.0.1",
)

go_module(
    name = "procfs",
    install = ["..."],
    module = "github.com/prometheus/procfs",
    version = "v0.0.3",
    deps = [
        ":x_sync",
    ],
)

go_module(
    name = "prometheus-common",
    install = [
        "expfmt",
        "model",
        "internal/...",
        "log",
    ],
    module = "github.com/prometheus/common",
    version = "v0.6.0",
    deps = [
        ":client_model",
        ":golang_protobuf_extensions",
        ":kingpin",
        ":logrus",
        ":protobuf",
    ],
)

go_module(
    name = "golang_protobuf_extensions",
    install = ["..."],
    module = "github.com/matttproud/golang_protobuf_extensions",
    version = "c12348ce28de40eed0136aa2b644d0ee0650e56c",
    deps = [
        ":protobuf-v1",
    ],
)

go_module(
    name = "kingpin",
    module = "gopkg.in/alecthomas/kingpin.v2",
    version = "v2.2.6",
    deps = [
        ":template",
        ":units",
    ],
)

go_module(
    name = "pgx.v5",
    install = ["..."],
    licences = ["MIT"],
    module = "github.com/jackc/pgx/v5",
    version = "v5.4.3",
    visibility = ["PUBLIC"],
)

go_module(
    name = "scany.v2",
    install = [
        "dbscan",
        "pgxscan",
    ],
    licences = ["MIT"],
    module = "github.com/georgysavva/scany/v2",
    version = "v2.1.3",
    visibility = ["PUBLIC"],
    deps = [":pgx.v5"],
)

go_module(
    name = "puddle",
    licences = ["MIT"],
    module = "github.com/jackc/puddle",
    version = "v1.1.3",
    visibility = ["PUBLIC"],
)

go_module(
    name = "pgconn",
    install = [
        ".",
        "internal/ctxwatch",
        "stmtcache",
    ],
    licences = ["MIT"],
    module = "github.com/jackc/pgconn",
    version = "v1.10.0",
    visibility = ["PUBLIC"],
    deps = [
        ":chunkreader.v2",
        ":pgio",
        ":pgpassfile",
        ":pgproto3.v2",
        ":pgservicefile",
        ":x_crypto",
        ":x_text",
    ],
)

go_module(
    name = "chunkreader.v2",
    licences = ["MIT"],
    module = "github.com/jackc/chunkreader/v2",
    version = "v2.0.1",
    visibility = ["PUBLIC"],
)

go_module(
    name = "pgpassfile",
    licences = ["MIT"],
    module = "github.com/jackc/pgpassfile",
    version = "v1.0.0",
    visibility = ["PUBLIC"],
)

go_module(
    name = "pgio",
    licences = ["MIT"],
    module = "github.com/jackc/pgio",
    version = "v1.0.0",
    visibility = ["PUBLIC"],
)

go_module(
    name = "pgtype",
    install = [
        "",
        "ext/...",
    ],
    licences = ["MIT"],
    module = "github.com/jackc/pgtype",
    version = "v1.10.0",
    visibility = ["PUBLIC"],
    deps = [
        ":decimal",
        ":gofrs-uuid",
        ":pgconn",
        ":pgio",
    ],
)

go_module(
    name = "pgservicefile",
    licences = ["MIT"],
    module = "github.com/jackc/pgservicefile",
    version = "v0.0.0-20200714003250-2b9c44734f2b",
    visibility = ["PUBLIC"],
)

go_module(
    name = "pgproto3.v2",
    licences = ["MIT"],
    module = "github.com/jackc/pgproto3/v2",
    version = "v2.1.1",
    visibility = ["PUBLIC"],
    deps = [
        ":chunkreader.v2",
        ":pgio",
    ],
)


go_module(
    name = "logfmt",
    licences = ["MIT"],
    module = "github.com/go-logfmt/logfmt",
    version = "v0.3.0",
    visibility = ["PUBLIC"],
)

go_module(
    name = "go-kit",
    install = [
        "log",
        "log/level",
    ],
    licences = ["MIT"],
    module = "github.com/go-kit/kit",
    version = "v0.11.0",
    visibility = ["PUBLIC"],
    deps = [":logfmt"],
)

go_module(
    name = "go-kit-log",
    install = ["..."],
    licences = ["MIT"],
    module = "github.com/go-kit/log",
    version = "v0.1.0",
    visibility = ["PUBLIC"],
    deps = [":logfmt"],
)

go_module(
    name = "log15.v2",
    licences = ["Apache-2.0"],
    module = "gopkg.in/inconshreveable/log15.v2",
    version = "v2.0.0-20180818164646-67afb5ed74ec",
    visibility = ["PUBLIC"],
    deps = [
        ":go-colorable",
        ":go-isatty",
        ":stack",
    ],
)

go_module(
    name = "xid",
    licences = ["MIT"],
    module = "github.com/rs/xid",
    version = "v1.3.0",
    visibility = ["PUBLIC"],
)

go_module(
    name = "zerolog",
    install = ["..."],
    licences = ["MIT"],
    module = "github.com/rs/zerolog",
    strip = [
        "cmd/lint",
        "journald",
    ],
    version = "v1.21.0",
    visibility = ["PUBLIC"],
    deps = [
        ":errors",
        ":xid",
    ],
)

go_module(
    name = "template",
    module = "github.com/alecthomas/template",
    version = "a0175ee3bccc567396460bf5acd36800cb10c49c",
)

go_module(
    name = "units",
    module = "github.com/alecthomas/units",
    version = "2efee857e7cfd4f3d0138cc3cbb1b4966962b93a",
)

go_module(
    name = "errors",
    module = "github.com/pkg/errors",
    version = "v0.9.1",
)

go_module(
    name = "decimal",
    module = "github.com/shopspring/decimal",
    version = "v1.3.1",
)

go_module(
    name = "gofrs-uuid",
    module = "github.com/gofrs/uuid",
    version = "v4.0.0",
)

go_module(
    name = "logrus",
    module = "github.com/sirupsen/logrus",
    strip = [
        "internal/testutils",
    ],
    version = "v1.8.1",
    deps = [
        ":x_crypto",
        ":x_sys",
    ],
)

go_module(
    name = "testify",
    install = [
        "assert",
        "require",
        "mock",
        "suite",
    ],
    module = "github.com/stretchr/testify",
    version = "v1.7.0",
    deps = [
        ":difflib",
        ":objx",
        ":spew",
        ":yaml.v3",
        ":yaml-v2",
    ],
)

go_module(
    name = "difflib",
    install = ["difflib"],
    module = "github.com/pmezard/go-difflib",
    version = "v1.0.0",
)

go_module(
    name = "yaml-v2",
    module = "gopkg.in/yaml.v2",
    version = "v2.2.2",
)

go_module(
    name = "yaml.v3",
    module = "gopkg.in/yaml.v3",
    version = "9f266ea9e77c",
)

go_module(
    name = "objx",
    module = "github.com/stretchr/objx",
    strip = ["vendor"],
    version = "v0.1.0",
)

go_module(
    name = "spew",
    install = ["spew"],
    module = "github.com/davecgh/go-spew",
    version = "v1.1.0",
)

go_module(
    name = "zap",
    install = ["..."],
    module = "go.uber.org/zap",
    version = "v1.17.0",
    deps = [
        ":atomic",
        ":multierr",
    ],
)

go_module(
    name = "multierr",
    module = "go.uber.org/multierr",
    version = "v1.7.0",
    deps = [":atomic"],
)

go_module(
    name = "atomic",
    module = "go.uber.org/atomic",
    version = "v1.7.0",
)

go_module(
    name = "go-colorable",
    module = "github.com/mattn/go-colorable",
    version = "v0.1.8",
    deps = [
        ":go-isatty",
    ],
)

go_module(
    name = "go-isatty",
    module = "github.com/mattn/go-isatty",
    version = "v0.0.13",
    deps = [":x_sys"],
)

go_module(
    name = "stack",
    module = "github.com/go-stack/stack",
    version = "v1.8.0",
)
