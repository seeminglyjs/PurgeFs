use clap::{Parser, Subcommand};

/// Fast terminal disk cleaner for macOS/Linux.
#[derive(Parser)]
#[command(name = "purgefs", version, about, long_about = None)]
struct Cli {
    #[command(subcommand)]
    command: Option<Commands>,
}

#[derive(Subcommand)]
enum Commands {
    /// Scan a path and report junk files/directories (no deletion).
    Scan {
        /// Root path to scan (default: current directory).
        #[arg(default_value = ".")]
        path: String,
    },
    /// Purge junk found under a path (asks before deleting).
    Purge {
        /// Root path to purge (default: current directory).
        #[arg(default_value = ".")]
        path: String,
        /// Skip confirmation prompt.
        #[arg(long)]
        yes: bool,
    },
}

fn main() {
    let cli = Cli::parse();

    match cli.command {
        Some(Commands::Scan { path }) => {
            println!("[scan] {path} — not implemented yet");
        }
        Some(Commands::Purge { path, yes }) => {
            println!("[purge] {path} (yes={yes}) — not implemented yet");
        }
        None => {
            println!("purgefs — run `purgefs --help`");
        }
    }
}
