import { Component, type ReactNode } from "react";
import { createRoot } from "react-dom/client";
import { CacheProvider } from "@emotion/react";
import createCache from "@emotion/cache";
import { Alert, Box, Button, Container, CssBaseline, Link, ThemeProvider, Typography } from "@mui/material";
import { theme } from "./theme";
import { HumanVerification } from "./HumanVerification";
import { EligibilityCard } from "./EligibilityCard";
import { ManualPaymentPanel } from "./ManualPaymentPanel";

class Boundary extends Component<{children: ReactNode}, {failed: boolean}> {
  state = { failed: false };
  static getDerivedStateFromError() { return { failed: true }; }
  render() {
    return this.state.failed ? <Container sx={{py: 6}}><Alert severity="error">页面暂时无法显示。若已打开付款链接，请先在原付款页面核实状态。</Alert><Button href="/" sx={{mt: 2}}>重新打开</Button></Container> : this.props.children;
  }
}

const cache = createCache({ key: "xgift", nonce: document.querySelector<HTMLMetaElement>('meta[name="csp-nonce"]')?.content, prepend: true });
createRoot(document.getElementById("root")!).render(
  <CacheProvider value={cache}><ThemeProvider theme={theme} defaultMode="system" modeStorageKey="xgift-mode"><CssBaseline />
    <Boundary>
      <Link href="#main" sx={{position: "absolute", top: -64, left: 16, p: 2, bgcolor: "background.paper", zIndex: 1500, "&:focus": {top: 8}}}>跳转到主要内容</Link>
      <Container component="main" id="main" maxWidth="lg" tabIndex={-1} sx={{py: {xs: 3, sm: 6}}}>
        <Box component="header" sx={{mb: {xs: 3, sm: 4}}}>
          <Typography component="h1" variant="h2" sx={{mb: 1}}>X Premium 赠送</Typography>
          <Typography color="text.secondary">先检测赠送资格，再生成链接前往 Stripe 自行付款。</Typography>
        </Box>
        <Box sx={{display: "grid", gridTemplateColumns: {xs: "minmax(0, 1fr)", md: "minmax(280px, 1fr) minmax(0, 2fr)"}, gap: 3, alignItems: "start", "& > *": {minWidth: 0}}}>
          <EligibilityCard lite />
          <ManualPaymentPanel publicMode />
        </Box>
        <Box component="footer" sx={{mt: 5, pt: 2, borderTop: 1, borderColor: "divider", color: "text.secondary", textAlign: "center"}}>
          <Typography variant="caption">XGift Lite · 基于 <Link href="https://github.com/mizorewww/x_gift_bot" target="_blank" rel="noopener noreferrer">mizorewww/x_gift_bot</Link></Typography>
        </Box>
      </Container>
      <HumanVerification />
    </Boundary>
  </ThemeProvider></CacheProvider>,
);
