import "./Footer.css";

type FooterProps = {
    buildSha?: string;
};

function Footer({ buildSha }: FooterProps) {
    return (
        <footer className="footer">
            <hr className="footer-divider" />
            <div className="footer-content">
                <span>
                    © {new Date().getFullYear()} <strong>Kube-JIT</strong>
                </span>
                <span className="footer-separator">|</span>
                <a
                    href="https://github.com/samirtahir91/kube-jit"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="footer-link"
                >
                    GitHub
                </a>
                <span className="footer-separator">|</span>
                <span>Apache 2.0 Licensed</span>
                {buildSha && (
                    <>
                        <span className="footer-separator">|</span>
                        <a
                            href={`https://github.com/samirtahir91/kube-jit/commit/${buildSha}`}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="footer-link"
                            style={{ fontSize: "0.95em", color: "#888" }}
                        >
                            Build: <code>{buildSha.slice(0, 7)}</code>
                        </a>
                    </>
                )}
            </div>
        </footer>
    );
}

export default Footer;