
import math
import sys

# Inspired by:
#   https://github.com/malb/dgs/blob/master/dgs/dgs_gauss_dp.c
def gen_cdf(sigma, skip):
    # Compute tables up to tau stddevs beyond the mean
    tau = 20
    upper_bound = math.ceil(sigma * tau) + 1;
    upper_bound_minus_one = upper_bound - 1;
    two_upper_bound_minus_one = 2 * upper_bound - 1;
    f = -1.0 / (2.0 * (sigma * sigma));

    rho = []
    for x in range(0, upper_bound, skip):
        v = math.e ** (x * x * f)
        rho.append(v)
    rho[0] /= 2.0;

    return rho

def print_cdf(sigma, skip, cdf):
    out = ""
    strings = ["%lg" % x for x in cdf]
    for i in range(0, len(strings), 5):
        out += "  " + ", ".join(strings[i:i+5]) + ",\n"

    print("package lwe\n")
    print("// CDF for Discrete Gaussian")
    print("//    Generated by %s" % sys.argv[0])
    print("// sigma = %g" % sigma)
    print("// print every %d entries\n\n" % skip)
    print("var cdf_skip64 = int(%d)\n" % skip)
    print("var cdf_table64 = [...]float64{\n%s}\n" % out)

def main():
    if len(sys.argv) != 3:
        sys.stderr.write("Usage: %s sigma skip\n" % sys.argv[0])
        sys.stderr.write("\tsigma = standard deviation\n")
        sys.stderr.write("\tskip  = print every k entries of CDF\n")
        sys.exit(-1)

    sigma = float(sys.argv[1])
    skip = int(sys.argv[2])
    
    cdf = gen_cdf(sigma, skip)
    print_cdf(sigma, skip, cdf)

if __name__ == "__main__":
    main()
