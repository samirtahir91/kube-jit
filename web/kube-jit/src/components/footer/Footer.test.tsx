import { render, screen } from "@testing-library/react";
import Footer from "./Footer";
import { describe, it, expect } from "vitest";

// kube-jit-gh-teams/web/kube-jit/src/components/footer/Footer.test.tsx

describe("Footer", () => {
    it("renders copyright, project name, GitHub link, and license", () => {
        render(<Footer />);
        const year = new Date().getFullYear();
        expect(screen.getByText(`© ${year}`)).toBeInTheDocument();
        expect(screen.getByText("Kube-JIT")).toBeInTheDocument();
        expect(screen.getByRole("link", { name: "GitHub" })).toHaveAttribute(
            "href",
            "https://github.com/samirtahir91/kube-jit"
        );
        expect(screen.getByText("Apache 2.0 Licensed")).toBeInTheDocument();
    });

    it("renders buildSha when provided", () => {
        const sha = "abcdef1234567890";
        render(<Footer buildSha={sha} />);
        // Should show only first 7 chars
        expect(screen.getByText("Build:")).toBeInTheDocument();
        expect(screen.getByText(sha.slice(0, 7))).toBeInTheDocument();
        const buildLink = screen.getByRole("link", { name: /Build:/ });
        expect(buildLink).toHaveAttribute(
            "href",
            `https://github.com/samirtahir91/kube-jit/commit/${sha}`
        );
    });

    it("does not render buildSha section when buildSha is not provided", () => {
        render(<Footer />);
        expect(screen.queryByText("Build:")).not.toBeInTheDocument();
    });

    it("renders correct number of separators", () => {
        // Without buildSha: 2 separators
        const { rerender } = render(<Footer />);
        expect(screen.getAllByText("|")).toHaveLength(2);

        // With buildSha: 3 separators
        rerender(<Footer buildSha="abc1234" />);
        expect(screen.getAllByText("|")).toHaveLength(3);
    });
});