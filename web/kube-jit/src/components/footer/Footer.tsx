import "./Footer.css";

function Footer() {
    return (
        <footer className="footer">
            <hr className="footer-divider" />
            <div className="footer-content">
                <span>
                    © {new Date().getFullYear()} <strong>Kube-JIT</strong>
                </span>
                <span className="footer-separator">|</span>
                <a
                    href="https://github.com/YOUR_GITHUB_ORG/kube-jit"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="footer-link"
                >
                    GitHub
                </a>
                <span className="footer-separator">|</span>
                <span>Apache 2.0 Licensed</span>
            </div>
        </footer>
    );
}

export default Footer;