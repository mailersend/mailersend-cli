use std::process::Command;

#[test]
fn answers_dynamic_completion_requests() {
    let out = Command::new(env!("CARGO_BIN_EXE_mailersend"))
        .env("MAILERSEND_COMPLETE", "fish")
        .args(["--", "mailersend", "comp"])
        .output()
        .unwrap();
    assert!(out.status.success(), "{out:?}");
    let lines = String::from_utf8(out.stdout).unwrap();
    assert!(
        lines.lines().any(|l| l.starts_with("completion\t")),
        "{lines}"
    );
}
