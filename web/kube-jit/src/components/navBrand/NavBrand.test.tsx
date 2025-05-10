import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import NavBrand from './NavBrand';

// Mock the image import
vi.mock('../../assets/login-logo.png', () => ({
  default: 'mock-login-logo.png',
}));

describe('NavBrand', () => {
  it('renders the Navbar.Brand with correct text', () => {
    render(<NavBrand />);
    expect(screen.getByText('Kube JIT')).toBeInTheDocument();
  });

  it('renders the logo image with correct attributes', () => {
    render(<NavBrand />);
    const img = screen.getByAltText('Login Logo') as HTMLImageElement;
    expect(img).toBeInTheDocument();
    expect(img.src).toContain('mock-login-logo.png');
    expect(img.width).toBe(30);
    expect(img.height).toBe(30);
    expect(img.className).toContain('me-2');
  });

  it('applies correct classes to Navbar.Brand', () => {
    render(<NavBrand />);
    const brand = screen.getByText('Kube JIT').closest('.navbar-brand');
    expect(brand).toHaveClass('text-light');
    expect(brand).toHaveClass('fw-bold');
  });
});