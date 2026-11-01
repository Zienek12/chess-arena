use std::time::Duration;

use crate::board::Color;

/// Time kept back from the clock for process and pipe latency.
const MOVE_OVERHEAD: Duration = Duration::from_millis(50);
/// Moves assumed to remain when the GUI does not send `movestogo`.
const DEFAULT_MOVES_TO_GO: u32 = 30;

/// Limits from the UCI `go` command.
#[derive(Clone, Debug, Default, PartialEq, Eq)]
pub struct SearchLimits {
    pub movetime: Option<Duration>,
    pub depth: Option<u32>,
    pub nodes: Option<u64>,
    pub wtime: Option<Duration>,
    pub btime: Option<Duration>,
    pub winc: Option<Duration>,
    pub binc: Option<Duration>,
    pub movestogo: Option<u32>,
    /// Search until `stop`; also set by `go ponder`.
    pub infinite: bool,
}

impl SearchLimits {
    /// Parses the arguments after `go`, for example `movetime 1000` or
    /// `wtime 60000 btime 60000 winc 1000 binc 1000`.
    pub fn parse(arguments: &[&str]) -> Result<Self, String> {
        let mut limits = Self::default();
        let mut tokens = arguments.iter();
        while let Some(&token) = tokens.next() {
            let mut value = || {
                tokens
                    .next()
                    .copied()
                    .ok_or_else(|| format!("go {} requires a value", token))
            };
            match token {
                "movetime" => limits.movetime = Some(millis(token, value()?)?),
                "depth" => limits.depth = Some(number(token, value()?)?),
                "nodes" => limits.nodes = Some(number(token, value()?)?),
                "wtime" => limits.wtime = Some(millis(token, value()?)?),
                "btime" => limits.btime = Some(millis(token, value()?)?),
                "winc" => limits.winc = Some(millis(token, value()?)?),
                "binc" => limits.binc = Some(millis(token, value()?)?),
                "movestogo" => limits.movestogo = Some(number(token, value()?)?),
                "infinite" | "ponder" => limits.infinite = true,
                _ => return Err(format!("unsupported go parameter {}", token)),
            }
        }
        Ok(limits)
    }

    /// How long the side to move may think, or `None` when only depth,
    /// nodes or `stop` end the search.
    pub fn time_budget(&self, side: Color) -> Option<Duration> {
        if self.infinite {
            return None;
        }
        if let Some(movetime) = self.movetime {
            return Some(movetime);
        }
        let (time, increment) = match side {
            Color::White => (self.wtime?, self.winc.unwrap_or_default()),
            Color::Black => (self.btime?, self.binc.unwrap_or_default()),
        };
        let moves_to_go = self.movestogo.unwrap_or(DEFAULT_MOVES_TO_GO).max(1);
        let budget = time / moves_to_go + increment * 3 / 4;
        let available = time.saturating_sub(MOVE_OVERHEAD);
        Some(budget.min(available).max(Duration::from_millis(1)))
    }
}

fn number<T: std::str::FromStr>(name: &str, value: &str) -> Result<T, String> {
    value
        .parse()
        .map_err(|_| format!("invalid go {} value {}", name, value))
}

/// Clock values may be negative when a GUI reports an overstepped clock.
fn millis(name: &str, value: &str) -> Result<Duration, String> {
    let millis: i64 = number(name, value)?;
    Ok(Duration::from_millis(millis.max(0) as u64))
}

#[cfg(test)]
mod tests {
    use super::*;

    fn parse(arguments: &str) -> SearchLimits {
        let tokens: Vec<&str> = arguments.split_whitespace().collect();
        SearchLimits::parse(&tokens).unwrap()
    }

    #[test]
    fn parses_fixed_limits() {
        let limits = parse("movetime 1500 depth 6 nodes 100000");
        assert_eq!(limits.movetime, Some(Duration::from_millis(1500)));
        assert_eq!(limits.depth, Some(6));
        assert_eq!(limits.nodes, Some(100_000));
        assert_eq!(
            limits.time_budget(Color::White),
            Some(Duration::from_millis(1500))
        );
    }

    #[test]
    fn budgets_clock_time_per_side() {
        let limits = parse("wtime 60000 btime 3000 winc 1000 binc 0");
        // 60000 / 30 + 1000 * 3/4
        assert_eq!(
            limits.time_budget(Color::White),
            Some(Duration::from_millis(2750))
        );
        assert_eq!(
            limits.time_budget(Color::Black),
            Some(Duration::from_millis(100))
        );
    }

    #[test]
    fn never_budgets_more_than_the_clock_holds() {
        let limits = parse("wtime 40 winc 5000");
        assert_eq!(
            limits.time_budget(Color::White),
            Some(Duration::from_millis(1))
        );
    }

    #[test]
    fn infinite_and_bare_go_have_no_budget() {
        assert_eq!(parse("infinite").time_budget(Color::White), None);
        assert_eq!(parse("").time_budget(Color::Black), None);
        assert_eq!(parse("depth 3").time_budget(Color::White), None);
    }

    #[test]
    fn rejects_invalid_arguments() {
        assert!(SearchLimits::parse(&["movetime"]).is_err());
        assert!(SearchLimits::parse(&["depth", "deep"]).is_err());
        assert!(SearchLimits::parse(&["searchmoves", "e2e4"]).is_err());
    }
}
