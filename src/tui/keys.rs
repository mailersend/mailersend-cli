pub struct Binding {
    pub key: &'static str,
    pub desc: &'static str,
}

pub struct Section {
    pub title: &'static str,
    pub bindings: &'static [Binding],
}

pub const HELP_SECTIONS: [Section; 4] = [
    Section {
        title: "Navigation",
        bindings: &[
            Binding {
                key: "↑/k",
                desc: "up",
            },
            Binding {
                key: "↓/j",
                desc: "down",
            },
            Binding {
                key: "enter",
                desc: "select",
            },
            Binding {
                key: "esc",
                desc: "back",
            },
        ],
    },
    Section {
        title: "Actions",
        bindings: &[
            Binding {
                key: "tab",
                desc: "next panel",
            },
            Binding {
                key: "r",
                desc: "refresh",
            },
            Binding {
                key: "p",
                desc: "switch profile",
            },
            Binding {
                key: "?",
                desc: "help",
            },
        ],
    },
    Section {
        title: "Views",
        bindings: &[
            Binding {
                key: "1",
                desc: "domains",
            },
            Binding {
                key: "2",
                desc: "activity",
            },
            Binding {
                key: "3",
                desc: "analytics",
            },
            Binding {
                key: "4",
                desc: "messages",
            },
            Binding {
                key: "5",
                desc: "suppressions",
            },
        ],
    },
    Section {
        title: "General",
        bindings: &[Binding {
            key: "q",
            desc: "quit",
        }],
    },
];
